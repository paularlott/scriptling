package plugin

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/paularlott/scriptling/object"
)

type Registrar interface {
	RegisterLibrary(*object.Library)
}

type ScriptLibraryRegistrar interface {
	RegisterScriptLibrary(name string, script string) error
}

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
		if fn.Mode == ModeWrapper || fn.Mode == ModeScript {
			return true
		}
	}
	for _, cls := range metadata.Schema.Classes {
		if cls.Mode == ModeWrapper || cls.Mode == ModeScript {
			return true
		}
	}
	return false
}

func buildLibrarySource(metadata Metadata) string {
	var builder strings.Builder
	builder.WriteString("import scriptling.plugin\n\n")
	for _, fn := range metadata.Schema.Functions {
		switch fn.Mode {
		case ModeRPC:
			builder.WriteString("def ")
			builder.WriteString(fn.Name)
			builder.WriteString("(*args, **kwargs):\n")
			builder.WriteString("    return scriptling.plugin.call_function(")
			builder.WriteString(strconv.Quote(metadata.Name))
			builder.WriteString(", ")
			builder.WriteString(strconv.Quote(fn.Name))
			builder.WriteString(", *args, **kwargs)\n\n")
		case ModeWrapper, ModeScript:
			builder.WriteString(fn.Source)
			if fn.Source != "" && fn.Source[len(fn.Source)-1] != '\n' {
				builder.WriteByte('\n')
			}
			builder.WriteByte('\n')
		}
	}
	for _, cls := range metadata.Schema.Classes {
		switch cls.Mode {
		case ModeRPC:
			builder.WriteString("class ")
			builder.WriteString(cls.Name)
			builder.WriteString(":\n")
			builder.WriteString("    def __init__(self, *args, **kwargs):\n")
			builder.WriteString("        self._plugin_remote = scriptling.plugin._new_object(")
			builder.WriteString(strconv.Quote(metadata.Name))
			builder.WriteString(", ")
			builder.WriteString(strconv.Quote(cls.Name))
			builder.WriteString(", *args, **kwargs)\n")
			if len(cls.Methods) == 0 {
				builder.WriteString("        pass\n")
			}
			for _, method := range cls.Methods {
				builder.WriteString("    def ")
				builder.WriteString(method.Name)
				builder.WriteString("(self, *args, **kwargs):\n")
				builder.WriteString("        return scriptling.plugin.call_method(self._plugin_remote, ")
				builder.WriteString(strconv.Quote(method.Name))
				builder.WriteString(", *args, **kwargs)\n")
			}
			builder.WriteString("\n")
		case ModeWrapper, ModeScript:
			builder.WriteString(cls.Source)
			if cls.Source != "" && cls.Source[len(cls.Source)-1] != '\n' {
				builder.WriteByte('\n')
			}
			builder.WriteByte('\n')
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

func buildProxyClass(client *Client, library string, schema ClassSchema) *object.Class {
	methods := make(map[string]object.Object)
	className := schema.Name

	methods["__init__"] = &object.Builtin{
		Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) == 0 {
				return object.NewString("plugin class initialization requires self")
			}
			self, ok := args[0].(*object.Instance)
			if !ok {
				return object.NewString("plugin class initialization requires instance self")
			}
			if err := initPluginObject(ctx, self, client, library, className, kwargs, args[1:]...); err != nil {
				return object.NewString(err.Error())
			}
			return &object.Null{}
		},
	}

	for _, methodSchema := range schema.Methods {
		methodName := methodSchema.Name
		methods[methodName] = &object.Builtin{
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				if len(args) == 0 {
					return object.NewString("plugin method requires self")
				}
				self, ok := args[0].(*object.Instance)
				if !ok {
					return object.NewString("plugin method requires instance self")
				}
				remote, ok := remoteFromInstance(self)
				if !ok {
					return object.NewString("plugin method called on non-plugin instance")
				}
				return callPluginMethod(ctx, remote, methodName, kwargs, args[1:]...)
			},
			HelpText: methodSchema.Description,
		}
	}

	return &object.Class{Name: className, Methods: methods}
}

func callPluginFunction(ctx context.Context, client *Client, name string, kwargs object.Kwargs, args ...object.Object) object.Object {
	encodedArgs, callbacks, err := valuesFromObjectsForCall(ctx, client, args)
	if err != nil {
		return object.NewString(err.Error())
	}
	defer unregisterCallbacks(client, callbacks)
	encodedKwargs, err := valuesFromKwargs(kwargs)
	if err != nil {
		return object.NewString(err.Error())
	}
	result, err := client.CallFunction(ctx, name, encodedArgs, encodedKwargs)
	if err != nil {
		return object.NewString(err.Error())
	}
	obj, err := valueToObject(result)
	if err != nil {
		return object.NewString(err.Error())
	}
	return obj
}

func newPluginObject(ctx context.Context, client *Client, library, className string, kwargs object.Kwargs, args ...object.Object) object.Object {
	instance := &object.Instance{
		Class:  &object.Class{Name: className, Methods: map[string]object.Object{}},
		Fields: make(map[string]object.Object),
	}
	if err := initPluginObject(ctx, instance, client, library, className, kwargs, args...); err != nil {
		return object.NewString(err.Error())
	}
	return instance
}

func initPluginObject(ctx context.Context, instance *object.Instance, client *Client, library, className string, kwargs object.Kwargs, args ...object.Object) error {
	encodedArgs, callbacks, err := valuesFromObjectsForCall(ctx, client, args)
	if err != nil {
		return err
	}
	defer unregisterCallbacks(client, callbacks)
	encodedKwargs, err := valuesFromKwargs(kwargs)
	if err != nil {
		return err
	}
	ref, err := client.NewObject(ctx, className, encodedArgs, encodedKwargs)
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
		return object.NewString("plugin object has been released")
	}
	encodedArgs, callbacks, err := valuesFromObjectsForCall(ctx, remote.Client, args)
	if err != nil {
		return object.NewString(err.Error())
	}
	defer unregisterCallbacks(remote.Client, callbacks)
	encodedKwargs, err := valuesFromKwargs(kwargs)
	if err != nil {
		return object.NewString(err.Error())
	}
	result, err := remote.Client.CallMethod(ctx, remote.ID, name, encodedArgs, encodedKwargs)
	if err != nil {
		return object.NewString(err.Error())
	}
	obj, err := valueToObject(result)
	if err != nil {
		return object.NewString(err.Error())
	}
	return obj
}

func Release(obj object.Object) error {
	instance, ok := obj.(*object.Instance)
	if !ok {
		return fmt.Errorf("expected plugin instance")
	}
	remote, ok := remoteFromInstance(instance)
	if !ok {
		return fmt.Errorf("expected plugin instance")
	}
	return releaseRemote(remote, instance)
}

func releaseRemote(remote *remoteObject, instance *object.Instance) error {
	if remote.Released {
		return nil
	}
	remote.Released = true
	if instance != nil {
		_ = object.ClearGCReleaseHook(instance)
		delete(instance.Fields, remoteFieldName)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return remote.Client.DestroyObject(ctx, remote.ID)
}

func installRemoteFinalizer(instance *object.Instance, remote *remoteObject) {
	_ = object.SetGCReleaseHook(instance, func() {
		_ = releaseRemote(remote, nil)
	})
}

func unregisterCallbacks(client *Client, callbackIDs []string) {
	for _, callbackID := range callbackIDs {
		client.UnregisterCallback(callbackID)
	}
}
