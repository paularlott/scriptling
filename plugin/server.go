package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/paularlott/jsonrpc"
	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

type Server struct {
	name        string
	version     string
	description string
	functions   map[string]*funcEntry
	classes     map[string]*classEntry
	constants   map[string]Value
	objects     map[string]*serverObject
	objectsMu   sync.RWMutex
	nextObject  atomic.Int64

	jsonrpcServer *jsonrpc.Server // inbound registry for HTTP (no callback runtime)
	srvOnce       sync.Once
}

type funcEntry struct {
	builtin *object.Builtin
	source  string
}

type classEntry struct {
	class  *object.Class
	source string
}

type serverObject struct {
	class    *object.Class
	instance *object.Instance
	mu       sync.Mutex
}

// serverRuntime carries the per-connection state a plugin handler needs to make
// reverse-direction requests back to the host (host callbacks and log records)
// over the bidirectional stdio peer. It is attached to handler contexts via
// callbackRuntimeKey; HTTP requests have no runtime (callbacks are unavailable).
type serverRuntime struct {
	peer *jsonrpc.Peer

	// hadHandshake tracks whether this connection handshook, and shuttingDown
	// whether plugin.shutdown was received. RunIO finalises objects only at the
	// end of a real session (not between the many short RunIO calls tests make
	// against a long-lived server), and only after in-flight handlers have
	// flushed — so plugin.shutdown cannot race a concurrent method call.
	hadHandshake atomic.Bool
	shuttingDown atomic.Bool
}

func NewServer(name, version, description string) *Server {
	return &Server{
		name:        declaredLibraryName(name),
		version:     version,
		description: description,
		functions:   make(map[string]*funcEntry),
		classes:     make(map[string]*classEntry),
		constants:   make(map[string]Value),
		objects:     make(map[string]*serverObject),
	}
}

func (s *Server) RegisterFunc(name string, builder *object.FunctionBuilder) *Server {
	s.functions[name] = &funcEntry{
		builtin: &object.Builtin{Fn: builder.Build()},
	}
	return s
}

// RegisterBuiltin registers a raw builtin function directly, bypassing the
// FunctionBuilder reflection layer. Use this when the function is already an
// object.BuiltinFunction (e.g. a closure that wraps a script handler).
func (s *Server) RegisterBuiltin(name string, fn object.BuiltinFunction) *Server {
	s.functions[name] = &funcEntry{
		builtin: &object.Builtin{Fn: fn},
	}
	return s
}

func (s *Server) RegisterClass(builder *object.ClassBuilder) *Server {
	class := builder.Build()
	s.classes[class.Name] = &classEntry{
		class: class,
	}
	return s
}

// RegisterBuiltinClass registers a scriptling *object.Class directly, bypassing
// the ClassBuilder. Use this when the class was loaded from a scriptling module
// (e.g. via Scriptling.Eval) rather than built from Go code.
func (s *Server) RegisterBuiltinClass(name string, class *object.Class) *Server {
	s.classes[name] = &classEntry{class: class}
	return s
}

func (s *Server) Wrapper(name string, source string) *Server {
	if entry, ok := s.functions[name]; ok {
		entry.source = source
	} else if entry, ok := s.classes[name]; ok {
		entry.source = source
	}
	return s
}

func (s *Server) RegisterScriptFunc(name string, source string) *Server {
	s.functions[name] = &funcEntry{
		source: source,
	}
	return s
}

func (s *Server) RegisterScriptClass(name string, source string) *Server {
	s.classes[name] = &classEntry{
		source: source,
	}
	return s
}

func (s *Server) Constant(name string, value any) *Server {
	s.constants[name] = goValueToTransport(value)
	return s
}

func (s *Server) Run() error {
	return s.RunIO(os.Stdin, os.Stdout)
}

// ServeHTTP serves the Scriptling plugin JSON-RPC protocol over HTTP. Mount it
// at a path such as /json-rpc and load it with plugin.Manager.LoadURL or
// scriptling.plugin.load(..., scriptling=True).
//
// HTTP plugin transport supports normal plugin calls, object lifecycle, and
// batches. Host callbacks and plugin.Logger(ctx) require the bidirectional
// stdio transport and are not available over HTTP. Framing (POST handling,
// batches, notifications, error responses, 204 on no-response) is provided by
// jsonrpc.Server.ServeHTTP; this server only supplies the plugin methods.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.httpServer().ServeHTTP(w, r)
}

// httpServer returns the jsonrpc.Server used for HTTP, built once and reused.
// It carries no callback runtime since HTTP is unidirectional.
func (s *Server) httpServer() *jsonrpc.Server {
	s.srvOnce.Do(func() { s.jsonrpcServer = s.buildServer(nil) })
	return s.jsonrpcServer
}

// buildServer returns a jsonrpc.Server whose handlers dispatch the plugin
// protocol methods via [Server.dispatch]. When runtime is non-nil it is attached
// to each handler's context so plugin code can make host callbacks
// (callback.call) and emit log records (host.log) over the bidirectional stdio
// transport; nil runtime (HTTP) disables those (callbacks/logger error out).
func (s *Server) buildServer(runtime *serverRuntime) *jsonrpc.Server {
	srv := jsonrpc.NewServer()
	for _, method := range pluginMethods {
		m := method
		srv.Handle(m, func(ctx context.Context, params json.RawMessage) (any, error) {
			if runtime != nil {
				ctx = context.WithValue(ctx, callbackRuntimeKey{}, runtime)
				if m == "scriptling.handshake" {
					runtime.hadHandshake.Store(true)
				}
				if m == "plugin.shutdown" {
					runtime.shuttingDown.Store(true)
				}
			}
			result, err := s.dispatch(ctx, m, params)
			// HTTP has no session-end hook (each request is independent), so
			// finalise objects inline on shutdown. stdio defers destruction to
			// RunIO after all in-flight handlers have flushed, avoiding a race
			// between plugin.shutdown and concurrent method calls.
			if runtime == nil && m == "plugin.shutdown" && err == nil {
				s.destroyAllObjects()
			}
			return result, err
		})
	}
	return srv
}

// pluginMethods are the JSON-RPC methods every plugin server dispatches.
var pluginMethods = []string{
	"scriptling.handshake",
	"environment.open",
	"environment.close",
	"plugin.shutdown",
	"function.call",
	"object.new",
	"object.call_method",
	"object.destroy",
}

// RunIO serves the plugin protocol over a bidirectional stream (stdio). The
// jsonrpc.Peer handles framing, batch dispatch, notification detection and
// response correlation in both directions; dispatch runs in the peer's server.
//
// RunIO blocks until the input reaches EOF — normally because the host closed
// the child's stdin after plugin.shutdown (which destroys objects and returns a
// response). The server need not force-exit on shutdown: the response is
// written before the host closes stdin, so there is no flush-before-exit race.
// Object finalizers run on plugin.shutdown and again on EOF (idempotent).
func (s *Server) RunIO(input io.Reader, output io.Writer) error {
	runtime := &serverRuntime{}
	peer := jsonrpc.NewPeer(input, output, s.buildServer(runtime))
	runtime.peer = peer
	_ = peer.Serve()
	peer.Wait() // let in-flight handlers flush; a handler write error lands in peer.Err()
	// Finalise objects only at the end of a real session (one that handshook or
	// received plugin.shutdown), and only after handlers are quiesced. Tests
	// drive a long-lived server through many short RunIO calls and rely on
	// objects surviving between them.
	if runtime.hadHandshake.Load() || runtime.shuttingDown.Load() {
		s.destroyAllObjects()
	}
	if err := peer.Err(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// callCallback invokes a host callback by id over the stdio peer. It blocks
// until the host responds; the plugin protocol issues callbacks synchronously,
// so at most one callback per outer call is in flight.
func (r *serverRuntime) callCallback(ctx context.Context, params callbackCallParams) (Value, error) {
	if r.peer == nil {
		return Value{}, fmt.Errorf("callbacks require the stdio transport")
	}
	var result Value
	if err := r.peer.Client().Call(ctx, "callback.call", params, &result); err != nil {
		return Value{}, err
	}
	return result, nil
}

// dispatch routes a plugin protocol method to its handler. It is the single
// entry point invoked by the jsonrpc handlers registered in buildServer.
func (s *Server) dispatch(ctx context.Context, method string, params any) (any, error) {
	switch method {
	case "scriptling.handshake":
		return handshakeResult{
			Protocol:  ProtocolVersion,
			Transport: "json",
			Library: libraryInfo{
				Name:        s.name,
				Version:     s.version,
				Description: s.description,
			},
			Capabilities: []string{"remote_objects"},
			Schema:       s.schema(),
		}, nil
	case "environment.open", "environment.close":
		return nil, nil
	case "plugin.shutdown":
		// Object finalisation is deferred to RunIO (stdio) or the handler
		// wrapper (HTTP) so it cannot race concurrent calls.
		return nil, nil
	case "function.call":
		var p functionCallParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		return s.callFunction(ctx, p)
	case "object.new":
		var p objectNewParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		return s.newObject(ctx, p)
	case "object.call_method":
		var p methodCallParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		return s.callMethod(ctx, p)
	case "object.destroy":
		var p objectDestroyParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		return nil, s.destroyObject(p)
	default:
		return nil, fmt.Errorf("unknown method %s", method)
	}
}

func (s *Server) schema() Schema {
	functions := make([]FunctionSchema, 0, len(s.functions))
	for name, entry := range s.functions {
		fs := FunctionSchema{Name: name}
		if entry.source != "" {
			fs.Source = entry.source
		}
		functions = append(functions, fs)
	}
	classes := make([]ClassSchema, 0, len(s.classes))
	for name, entry := range s.classes {
		cs := ClassSchema{
			Name: name,
		}
		if entry.source != "" {
			cs.Source = entry.source
		}
		if entry.class != nil {
			cs.Constructor = FunctionSchema{Name: name}
			methods := make([]FunctionSchema, 0, len(entry.class.Methods))
			properties := make([]PropertySchema, 0)
			for mname, member := range entry.class.Methods {
				if mname == "__init__" || mname == "__del__" {
					continue
				}
				if property, ok := member.(*object.Property); ok {
					properties = append(properties, PropertySchema{
						Name:     mname,
						Settable: property.Setter != nil,
					})
					continue
				}
				methods = append(methods, FunctionSchema{Name: mname})
			}
			cs.Methods = methods
			cs.Properties = properties
		}
		classes = append(classes, cs)
	}
	constants := make([]ConstantSchema, 0, len(s.constants))
	for name, value := range s.constants {
		constants = append(constants, ConstantSchema{Name: name, Value: value})
	}
	return Schema{Functions: functions, Classes: classes, Constants: constants}
}

func (s *Server) callFunction(ctx context.Context, params functionCallParams) (Value, error) {
	entry, ok := s.functions[params.Name]
	if !ok || entry.builtin == nil {
		return Value{}, fmt.Errorf("unknown function %s (available: %s)", params.Name, availableMapKeys(s.functions))
	}
	args, err := transportValuesToObjects(params.Args)
	if err != nil {
		return Value{}, err
	}
	kwargs, err := transportKwargsToObjects(params.Kwargs)
	if err != nil {
		return Value{}, err
	}
	result := entry.builtin.Fn(ctx, object.NewKwargs(kwargs), args...)
	if errObj, ok := result.(*object.Error); ok {
		return Value{}, errors.New(errObj.Message)
	}
	return objectToValue(result)
}

func (s *Server) newObject(ctx context.Context, params objectNewParams) (*RemoteRef, error) {
	entry, ok := s.classes[params.Class]
	if !ok || entry.class == nil {
		return nil, fmt.Errorf("unknown class %s (available: %s)", params.Class, availableMapKeys(s.classes))
	}
	class := entry.class
	instance := object.NewInstance(class)
	if init, ok := class.LookupMember("__init__"); ok {
		objArgs, err := transportValuesToObjects(params.Args)
		if err != nil {
			return nil, err
		}
		objKwargs, err := transportKwargsToObjects(params.Kwargs)
		if err != nil {
			return nil, err
		}
		callArgs := append([]object.Object{instance}, objArgs...)
		result := evaluator.ApplyFunctionGIL(ctx, init, callArgs, objKwargs, object.NewEnvironment())
		if errObj, ok := result.(*object.Error); ok {
			return nil, errors.New(errObj.Message)
		}
	}
	id := strconv.FormatInt(s.nextObject.Add(1), 10)
	s.objectsMu.Lock()
	s.objects[id] = &serverObject{
		class:    class,
		instance: instance,
	}
	s.objectsMu.Unlock()
	return &RemoteRef{
		Library: s.name,
		Class:   class.Name,
		ID:      id,
	}, nil
}

func (s *Server) callMethod(ctx context.Context, params methodCallParams) (Value, error) {
	remoteObject := s.loadObject(params.ObjectID)
	if remoteObject == nil {
		return Value{}, fmt.Errorf("unknown object %s", params.ObjectID)
	}
	remoteObject.mu.Lock()
	defer remoteObject.mu.Unlock()
	instance := remoteObject.instance
	class := remoteObject.class
	if instance == nil || class == nil {
		return Value{}, fmt.Errorf("unknown object %s", params.ObjectID)
	}
	methodObj, ok := class.LookupMember(params.Method)
	if !ok {
		return Value{}, fmt.Errorf("unknown method %s on %s (available: %s)", params.Method, class.Name, availableObjectMapKeys(class.Methods))
	}
	if property, ok := methodObj.(*object.Property); ok {
		return s.callProperty(ctx, property, instance, params)
	}
	objArgs, err := transportValuesToObjects(params.Args)
	if err != nil {
		return Value{}, err
	}
	objKwargs, err := transportKwargsToObjects(params.Kwargs)
	if err != nil {
		return Value{}, err
	}
	callArgs := append([]object.Object{instance}, objArgs...)
	result := evaluator.ApplyFunctionGIL(ctx, methodObj, callArgs, objKwargs, object.NewEnvironment())
	if errObj, ok := result.(*object.Error); ok {
		return Value{}, errors.New(errObj.Message)
	}
	return objectToValue(result)
}

func (s *Server) callProperty(ctx context.Context, property *object.Property, instance *object.Instance, params methodCallParams) (Value, error) {
	objArgs, err := transportValuesToObjects(params.Args)
	if err != nil {
		return Value{}, err
	}
	objKwargs, err := transportKwargsToObjects(params.Kwargs)
	if err != nil {
		return Value{}, err
	}
	var target object.Object
	var callArgs []object.Object
	switch len(objArgs) {
	case 0:
		if len(objKwargs) != 0 {
			return Value{}, fmt.Errorf("property %s getter does not accept keyword arguments", params.Method)
		}
		if property.Getter == nil {
			return Value{}, fmt.Errorf("property %s is write-only", params.Method)
		}
		target = property.Getter
		callArgs = []object.Object{instance}
	case 1:
		if property.Setter == nil {
			return Value{}, fmt.Errorf("can't set attribute '%s': property is read-only", params.Method)
		}
		target = property.Setter
		callArgs = []object.Object{instance, objArgs[0]}
	default:
		return Value{}, fmt.Errorf("property %s expects zero arguments for get or one argument for set", params.Method)
	}
	result := evaluator.ApplyFunctionGIL(ctx, target, callArgs, objKwargs, object.NewEnvironment())
	if errObj, ok := result.(*object.Error); ok {
		return Value{}, errors.New(errObj.Message)
	}
	return objectToValue(result)
}

func availableMapKeys[T any](items map[string]T) string {
	if len(items) == 0 {
		return "none"
	}
	names := make([]string, 0, len(items))
	for name := range items {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func availableObjectMapKeys(items map[string]object.Object) string {
	if len(items) == 0 {
		return "none"
	}
	names := make([]string, 0, len(items))
	for name := range items {
		if name == "__init__" || name == "__del__" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "none"
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

// ObjectCount returns the number of live server-side objects. Useful in tests.
func (s *Server) ObjectCount() int {
	s.objectsMu.RLock()
	defer s.objectsMu.RUnlock()
	return len(s.objects)
}

// destroyAllObjects calls __del__ on every remaining server-side object and
// empties the map. Called when a stdio connection closes or plugin.shutdown is
// received so that instances are not silently leaked.
func (s *Server) destroyAllObjects() {
	s.objectsMu.Lock()
	remaining := s.objects
	s.objects = make(map[string]*serverObject)
	s.objectsMu.Unlock()
	for _, obj := range remaining {
		obj.mu.Lock()
		if obj.instance != nil && obj.class != nil {
			if del, ok := obj.class.LookupMember("__del__"); ok {
				evaluator.ApplyFunctionGIL(context.Background(), del, []object.Object{obj.instance}, nil, object.NewEnvironment())
			}
			obj.instance = nil
			obj.class = nil
		}
		obj.mu.Unlock()
	}
}

func (s *Server) destroyObject(params objectDestroyParams) error {
	s.objectsMu.Lock()
	remoteObject, ok := s.objects[params.ObjectID]
	if ok {
		delete(s.objects, params.ObjectID)
	}
	s.objectsMu.Unlock()
	if !ok || remoteObject == nil {
		return nil
	}
	remoteObject.mu.Lock()
	defer remoteObject.mu.Unlock()
	if remoteObject.instance != nil && remoteObject.class != nil {
		if del, exists := remoteObject.class.LookupMember("__del__"); exists {
			evaluator.ApplyFunctionGIL(context.Background(), del, []object.Object{remoteObject.instance}, nil, object.NewEnvironment())
		}
		remoteObject.instance = nil
		remoteObject.class = nil
	}
	return nil
}

func (s *Server) loadObject(objectID string) *serverObject {
	s.objectsMu.RLock()
	defer s.objectsMu.RUnlock()
	return s.objects[objectID]
}

func decodeParams(params any, target any) error {
	raw, err := json.Marshal(params)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func transportValuesToObjects(values []Value) ([]object.Object, error) {
	args := make([]object.Object, 0, len(values))
	for _, value := range values {
		obj, err := valueToObject(value)
		if err != nil {
			return nil, err
		}
		args = append(args, obj)
	}
	return args, nil
}

func transportKwargsToObjects(values map[string]Value) (map[string]object.Object, error) {
	if len(values) == 0 {
		return nil, nil
	}
	out := make(map[string]object.Object, len(values))
	for key, value := range values {
		obj, err := valueToObject(value)
		if err != nil {
			return nil, err
		}
		out[key] = obj
	}
	return out, nil
}

func goValueToTransport(value any) Value {
	switch v := value.(type) {
	case nil:
		return Value{Type: valueNull}
	case error:
		return Value{Type: valueString, Value: v.Error()}
	case object.Object:
		encoded, err := objectToValue(v)
		if err == nil {
			return encoded
		}
		return Value{Type: valueString, Value: err.Error()}
	case bool:
		return Value{Type: valueBool, Value: v}
	case int:
		return Value{Type: valueInt, Value: int64(v)}
	case int64:
		return Value{Type: valueInt, Value: v}
	case float64:
		return Value{Type: valueFloat, Value: v}
	case string:
		return Value{Type: valueString, Value: v}
	case []any:
		items := make([]Value, 0, len(v))
		for _, item := range v {
			items = append(items, goValueToTransport(item))
		}
		return Value{Type: valueList, Items: items}
	case map[string]any:
		entries := make(map[string]Value, len(v))
		for key, item := range v {
			entries[key] = goValueToTransport(item)
		}
		return Value{Type: valueDict, Entries: entries}
	default:
		return goReflectValueToTransport(reflect.ValueOf(value))
	}
}

func goReflectValueToTransport(value reflect.Value) Value {
	if !value.IsValid() {
		return Value{Type: valueNull}
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return Value{Type: valueNull}
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Bool:
		return Value{Type: valueBool, Value: value.Bool()}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return Value{Type: valueInt, Value: value.Int()}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return Value{Type: valueInt, Value: int64(value.Uint())}
	case reflect.Float32, reflect.Float64:
		return Value{Type: valueFloat, Value: value.Float()}
	case reflect.String:
		return Value{Type: valueString, Value: value.String()}
	case reflect.Slice, reflect.Array:
		items := make([]Value, 0, value.Len())
		for i := 0; i < value.Len(); i++ {
			items = append(items, goReflectValueToTransport(value.Index(i)))
		}
		return Value{Type: valueList, Items: items}
	case reflect.Map:
		if value.Type().Key().Kind() != reflect.String {
			return Value{Type: valueString, Value: fmt.Sprint(value.Interface())}
		}
		entries := make(map[string]Value, value.Len())
		for _, key := range value.MapKeys() {
			entries[key.String()] = goReflectValueToTransport(value.MapIndex(key))
		}
		return Value{Type: valueDict, Entries: entries}
	case reflect.Struct:
		entries := make(map[string]Value, value.NumField())
		valueType := value.Type()
		for i := 0; i < value.NumField(); i++ {
			field := valueType.Field(i)
			if field.PkgPath != "" {
				continue
			}
			name := field.Name
			if tag := field.Tag.Get("json"); tag != "" {
				if tag == "-" {
					continue
				}
				for idx, ch := range tag {
					if ch == ',' {
						tag = tag[:idx]
						break
					}
				}
				if tag != "" {
					name = tag
				}
			}
			entries[name] = goReflectValueToTransport(value.Field(i))
		}
		return Value{Type: valueDict, Entries: entries}
	default:
		if value.CanInterface() {
			return Value{Type: valueString, Value: fmt.Sprint(value.Interface())}
		}
		return Value{Type: valueString, Value: fmt.Sprint(value)}
	}
}
