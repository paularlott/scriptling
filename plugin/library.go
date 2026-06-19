package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/object"
)

type Registrar interface {
	RegisterLibrary(*object.Library)
}

type ScriptLibraryRegistrar interface {
	RegisterScriptLibrary(name string, script string) error
}

// DefaultReleaseTimeout is used by Release and GC finalizers when no caller
// context is available. Use ReleaseWithContext for request-scoped cleanup.
const DefaultReleaseTimeout = 2 * time.Second

func RegisterLibraries(registrar Registrar, manager *Manager) {
	if manager == nil {
		return
	}
	registrar.RegisterLibrary(NewControlLibrary(manager))
	for _, metadata := range manager.List() {
		client, ok := manager.Get(metadata.Name)
		if !ok {
			continue
		}
		if needsScriptRegistration(metadata) {
			if scriptRegistrar, ok := registrar.(ScriptLibraryRegistrar); ok {
				_ = scriptRegistrar.RegisterScriptLibrary(metadata.Name, buildLibrarySource(metadata))
				continue
			}
		}
		registrar.RegisterLibrary(buildProxyLibrary(client))
	}
}

func needsScriptRegistration(metadata Metadata) bool {
	for _, fn := range metadata.Schema.Functions {
		if fn.Source != "" {
			return true
		}
	}
	for _, cls := range metadata.Schema.Classes {
		if cls.Source != "" {
			return true
		}
	}
	return false
}

func buildLibrarySource(metadata Metadata) string {
	var builder strings.Builder
	builder.WriteString("import scriptling.plugin\n\n")
	for _, fn := range metadata.Schema.Functions {
		if fn.Source != "" {
			builder.WriteString(fn.Source)
			if fn.Source[len(fn.Source)-1] != '\n' {
				builder.WriteByte('\n')
			}
			builder.WriteByte('\n')
		} else {
			builder.WriteString("def ")
			builder.WriteString(fn.Name)
			builder.WriteString("(*args, **kwargs):\n")
			builder.WriteString("    return scriptling.plugin.call_function(")
			builder.WriteString(strconv.Quote(metadata.Name))
			builder.WriteString(", ")
			builder.WriteString(strconv.Quote(fn.Name))
			builder.WriteString(", *args, **kwargs)\n\n")
		}
	}
	for _, cls := range metadata.Schema.Classes {
		if cls.Source != "" {
			builder.WriteString(cls.Source)
			if cls.Source[len(cls.Source)-1] != '\n' {
				builder.WriteByte('\n')
			}
			builder.WriteByte('\n')
		} else {
			builder.WriteString("class ")
			builder.WriteString(cls.Name)
			builder.WriteString(":\n")
			builder.WriteString("    def __init__(self, *args, **kwargs):\n")
			builder.WriteString("        self._plugin_remote = scriptling.plugin._new_object(")
			builder.WriteString(strconv.Quote(metadata.Name))
			builder.WriteString(", ")
			builder.WriteString(strconv.Quote(cls.Name))
			builder.WriteString(", *args, **kwargs)\n")
			for _, method := range cls.Methods {
				builder.WriteString("    def ")
				builder.WriteString(method.Name)
				builder.WriteString("(self, *args, **kwargs):\n")
				builder.WriteString("        return scriptling.plugin.call_method(self._plugin_remote, ")
				builder.WriteString(strconv.Quote(method.Name))
				builder.WriteString(", *args, **kwargs)\n")
			}
			for _, property := range cls.Properties {
				builder.WriteString("    @property\n")
				builder.WriteString("    def ")
				builder.WriteString(property.Name)
				builder.WriteString("(self):\n")
				builder.WriteString("        return scriptling.plugin.call_method(self._plugin_remote, ")
				builder.WriteString(strconv.Quote(property.Name))
				builder.WriteString(")\n")
				if property.Settable {
					builder.WriteString("    @")
					builder.WriteString(property.Name)
					builder.WriteString(".setter\n")
					builder.WriteString("    def ")
					builder.WriteString(property.Name)
					builder.WriteString("(self, value):\n")
					builder.WriteString("        return scriptling.plugin.call_method(self._plugin_remote, ")
					builder.WriteString(strconv.Quote(property.Name))
					builder.WriteString(", value)\n")
				}
			}
			builder.WriteString("    def __del__(self):\n")
			builder.WriteString("        scriptling.plugin.release(self._plugin_remote)\n")
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func buildProxyLibrary(client *Client) *object.Library {
	metadata := client.Metadata()
	functions := make(map[string]*object.Builtin)
	constants := make(map[string]object.Object)

	for _, fn := range metadata.Schema.Functions {
		name := fn.Name
		functions[name] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return callPluginFunction(ctx, client, name, kwargs, args...)
			},
			HelpText: fn.Description,
		}
	}

	for _, classSchema := range metadata.Schema.Classes {
		class := buildProxyClass(client, metadata.Name, classSchema)
		constants[classSchema.Name] = class
	}

	for _, constant := range metadata.Schema.Constants {
		obj, err := valueToObject(constant.Value)
		if err != nil {
			obj = object.NewString(err.Error())
		}
		constants[constant.Name] = obj
	}

	return object.NewLibrary(metadata.Name, functions, constants, metadata.Description)
}

func pluginErr(msg string) *object.Error {
	return &object.Error{Message: msg}
}

func pluginErrf(format string, args ...any) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, args...)}
}

func buildProxyClass(client *Client, library string, schema ClassSchema) *object.Class {
	methods := make(map[string]object.Object)
	className := schema.Name

	methods["__init__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return pluginErr("plugin class initialization requires self")
			}
			self, ok := args[0].(*object.Instance)
			if !ok {
				return pluginErr("plugin class initialization requires instance self")
			}
			if err := initPluginObject(ctx, self, client, library, className, kwargs, args[1:]...); err != nil {
				return pluginErr(err.Error())
			}
			return &object.Null{}
		},
	}

	for _, methodSchema := range schema.Methods {
		methodName := methodSchema.Name
		methods[methodName] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) == 0 {
					return pluginErr("plugin method requires self")
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return pluginErr("plugin method requires instance self")
				}
				remote, ok := remoteFromInstance(self)
				if !ok {
					return pluginErr("plugin method called on non-plugin instance")
				}
				return callPluginMethod(ctx, remote, methodName, kwargs, args[1:]...)
			},
			HelpText: methodSchema.Description,
		}
	}
	for _, propertySchema := range schema.Properties {
		propertyName := propertySchema.Name
		property := &object.Property{
			Getter: &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) == 0 {
						return pluginErr("plugin property requires self")
					}
					self, ok := args[0].(*object.Instance)
					if !ok {
						return pluginErr("plugin property requires instance self")
					}
					remote, ok := remoteFromInstance(self)
					if !ok {
						return pluginErr("plugin property called on non-plugin instance")
					}
					return callPluginMethod(ctx, remote, propertyName, object.Kwargs{}, args[1:]...)
				},
				HelpText: propertySchema.Description,
			},
		}
		if propertySchema.Settable {
			property.Setter = &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) == 0 {
						return pluginErr("plugin property setter requires self")
					}
					self, ok := args[0].(*object.Instance)
					if !ok {
						return pluginErr("plugin property setter requires instance self")
					}
					remote, ok := remoteFromInstance(self)
					if !ok {
						return pluginErr("plugin property setter called on non-plugin instance")
					}
					return callPluginMethod(ctx, remote, propertyName, kwargs, args[1:]...)
				},
				HelpText: propertySchema.Description,
			}
		}
		methods[propertyName] = property
	}

	return &object.Class{Name: className, Methods: methods}
}

func callPluginFunction(ctx context.Context, client *Client, name string, kwargs object.Kwargs, args ...object.Object) object.Object {
	if !client.HandshakeDone() {
		return callRawFunction(ctx, client, name, kwargs, args...)
	}
	callbacks := newCallbackSet()
	encodedArgs, err := valuesFromObjectsWithCallbacks(args, callbacks)
	if err != nil {
		return pluginErr(err.Error())
	}
	encodedKwargs, err := valuesFromKwargsWithCallbacks(kwargs, callbacks)
	if err != nil {
		return pluginErr(err.Error())
	}
	result, err := client.CallFunctionWithCallbacks(ctx, name, encodedArgs, encodedKwargs, callbacks)
	if err != nil {
		return pluginErr(err.Error())
	}
	obj, err := valueToObject(result)
	if err != nil {
		return pluginErr(err.Error())
	}
	return obj
}

type batchCallSpec struct {
	Name   string
	Args   []object.Object
	Kwargs object.Kwargs
}

func batchCallPluginFunctions(ctx context.Context, client *Client, calls []batchCallSpec) object.Object {
	if len(calls) == 0 {
		return &object.List{}
	}
	requests := make([]batchRequest, len(calls))
	for i, call := range calls {
		if err := rejectBatchCallbacks(call.Args, call.Kwargs); err != nil {
			return pluginErrf("batch call %d (%s): %v", i, call.Name, err)
		}
		if client.HandshakeDone() {
			encodedArgs, err := valuesFromObjects(call.Args)
			if err != nil {
				return pluginErrf("batch call %d (%s): %v", i, call.Name, err)
			}
			encodedKwargs, err := valuesFromKwargs(call.Kwargs)
			if err != nil {
				return pluginErrf("batch call %d (%s): %v", i, call.Name, err)
			}
			requests[i] = batchRequest{
				Method: "function.call",
				Params: functionCallParams{
					Name:   call.Name,
					Args:   encodedArgs,
					Kwargs: encodedKwargs,
				},
			}
			continue
		}
		params := rawParamsFromObjects(call.Kwargs, call.Args...)
		requests[i] = batchRequest{Method: call.Name, Params: params}
	}

	results, err := client.Batch(ctx, requests)
	if err != nil {
		return pluginErr(err.Error())
	}
	items := make([]object.Object, len(results))
	for i, raw := range results {
		if len(raw) == 0 {
			items[i] = &object.Null{}
			continue
		}
		if client.HandshakeDone() {
			var value Value
			if err := json.Unmarshal(raw, &value); err != nil {
				return pluginErrf("batch call %d (%s): failed to decode result: %v", i, calls[i].Name, err)
			}
			obj, err := valueToObject(value)
			if err != nil {
				return pluginErrf("batch call %d (%s): %v", i, calls[i].Name, err)
			}
			items[i] = obj
			continue
		}
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return pluginErrf("batch call %d (%s): failed to decode result: %v", i, calls[i].Name, err)
		}
		items[i] = conversion.FromGo(decoded)
	}
	return &object.List{Elements: items}
}

// callRawFunction sends a raw JSON-RPC request for a client that did not
// perform the plugin handshake. The function name goes directly on the wire
// as the JSON-RPC method (no function.call wrapper). Params are mapped from
// the Scriptling args/kwargs:
//   - kwargs present → params is a JSON object
//   - single positional arg, no kwargs → params is that value
//   - multiple positional args, no kwargs → params is a JSON array
//   - nothing → params omitted
func callRawFunction(ctx context.Context, client *Client, name string, kwargs object.Kwargs, args ...object.Object) object.Object {
	params := rawParamsFromObjects(kwargs, args...)
	var raw json.RawMessage
	if err := client.Call(ctx, name, params, &raw); err != nil {
		return pluginErr(err.Error())
	}
	if len(raw) == 0 {
		return &object.Null{}
	}
	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return pluginErrf("failed to decode result: %v", err)
	}
	return conversion.FromGo(decoded)
}

func rawParamsFromObjects(kwargs object.Kwargs, args ...object.Object) any {
	switch {
	case len(kwargs.Kwargs) > 0:
		m := make(map[string]any, len(kwargs.Kwargs))
		for k, v := range kwargs.Kwargs {
			m[k] = conversion.ToGo(v)
		}
		return m
	case len(args) == 1:
		return conversion.ToGo(args[0])
	case len(args) > 1:
		arr := make([]any, len(args))
		for i, a := range args {
			arr[i] = conversion.ToGo(a)
		}
		return arr
	default:
		return nil
	}
}

func rejectBatchCallbacks(args []object.Object, kwargs object.Kwargs) error {
	for _, arg := range args {
		if containsCallable(arg) {
			return fmt.Errorf("batch_call does not support callback arguments")
		}
	}
	for _, arg := range kwargs.Kwargs {
		if containsCallable(arg) {
			return fmt.Errorf("batch_call does not support callback arguments")
		}
	}
	return nil
}

func containsCallable(obj object.Object) bool {
	switch v := obj.(type) {
	case *object.Function, *object.LambdaFunction, *object.Builtin:
		return true
	case *object.List:
		for _, item := range v.Elements {
			if containsCallable(item) {
				return true
			}
		}
	case *object.Tuple:
		for _, item := range v.Elements {
			if containsCallable(item) {
				return true
			}
		}
	case *object.Dict:
		for _, pair := range v.Pairs {
			if containsCallable(pair.Value) {
				return true
			}
		}
	}
	return false
}

func newPluginObject(ctx context.Context, client *Client, library, className string, kwargs object.Kwargs, args ...object.Object) object.Object {
	instance := &object.Instance{
		Class:  &object.Class{Name: className, Methods: map[string]object.Object{}},
		Fields: make(map[string]object.Object),
	}
	if err := initPluginObject(ctx, instance, client, library, className, kwargs, args...); err != nil {
		return pluginErr(err.Error())
	}
	return instance
}

func initPluginObject(ctx context.Context, instance *object.Instance, client *Client, library, className string, kwargs object.Kwargs, args ...object.Object) error {
	callbacks := newCallbackSet()
	encodedArgs, err := valuesFromObjectsWithCallbacks(args, callbacks)
	if err != nil {
		return err
	}
	encodedKwargs, err := valuesFromKwargsWithCallbacks(kwargs, callbacks)
	if err != nil {
		return err
	}
	ref, err := client.NewObjectWithCallbacks(ctx, className, encodedArgs, encodedKwargs, callbacks)
	if err != nil {
		return err
	}
	remote := &remoteObject{
		Client:  client,
		Library: library,
		Class:   className,
		ID:      ref.ID,
	}
	if instance.Fields == nil {
		instance.Fields = make(map[string]object.Object)
	}
	instance.Fields[remoteFieldName] = &object.ClientWrapper{TypeName: className, Client: remote}
	installRemoteFinalizer(instance, remote)
	return nil
}

func callPluginMethod(ctx context.Context, remote *remoteObject, name string, kwargs object.Kwargs, args ...object.Object) object.Object {
	if remote.Released {
		return pluginErr("plugin object has been released")
	}
	callbacks := newCallbackSet()
	encodedArgs, err := valuesFromObjectsWithCallbacks(args, callbacks)
	if err != nil {
		return pluginErr(err.Error())
	}
	encodedKwargs, err := valuesFromKwargsWithCallbacks(kwargs, callbacks)
	if err != nil {
		return pluginErr(err.Error())
	}
	result, err := remote.Client.CallMethodWithCallbacks(ctx, remote.ID, name, encodedArgs, encodedKwargs, callbacks)
	if err != nil {
		return pluginErr(err.Error())
	}
	obj, err := valueToObject(result)
	if err != nil {
		return pluginErr(err.Error())
	}
	return obj
}

func Release(obj object.Object) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultReleaseTimeout)
	defer cancel()
	return ReleaseWithContext(ctx, obj)
}

// ReleaseWithContext explicitly releases a remote plugin object using ctx.
func ReleaseWithContext(ctx context.Context, obj object.Object) error {
	instance, ok := obj.(*object.Instance)
	if !ok {
		return fmt.Errorf("expected plugin instance")
	}
	remote, ok := remoteFromInstance(instance)
	if !ok {
		return fmt.Errorf("expected plugin instance")
	}
	return releaseRemote(ctx, remote, instance)
}

func releaseRemote(ctx context.Context, remote *remoteObject, instance *object.Instance) error {
	if remote.Released {
		return nil
	}
	remote.Released = true
	if instance != nil {
		_ = object.ClearGCReleaseHook(instance)
		delete(instance.Fields, remoteFieldName)
	}
	return remote.Client.DestroyObject(ctx, remote.ID)
}

func installRemoteFinalizer(instance *object.Instance, remote *remoteObject) {
	_ = object.SetGCReleaseHook(instance, func() {
		ctx, cancel := context.WithTimeout(context.Background(), DefaultReleaseTimeout)
		defer cancel()
		_ = releaseRemote(ctx, remote, nil)
	})
}
