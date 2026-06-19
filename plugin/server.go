package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

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

type serverRuntime struct {
	encoder *json.Encoder
	writeMu sync.Mutex
	nextID  atomic.Int64
	pending map[int64]chan rpcResponse
	mu      sync.Mutex
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

func (s *Server) RegisterClass(builder *object.ClassBuilder) *Server {
	class := builder.Build()
	s.classes[class.Name] = &classEntry{
		class: class,
	}
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

func (s *Server) RunIO(input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(bufio.NewReader(input))
	runtime := &serverRuntime{
		encoder: json.NewEncoder(output),
		pending: make(map[int64]chan rpcResponse),
	}
	var wg sync.WaitGroup
	var firstErr error
	var firstErrMu sync.Mutex
	recordErr := func(err error) {
		if err == nil {
			return
		}
		firstErrMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		firstErrMu.Unlock()
	}
	getErr := func() error {
		firstErrMu.Lock()
		defer firstErrMu.Unlock()
		return firstErr
	}

	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				wg.Wait()
				return getErr()
			}
			return err
		}
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			continue
		}
		if raw[0] == '[' {
			var batch []rpcMessage
			if err := json.Unmarshal(raw, &batch); err != nil {
				return err
			}
			var responses []rpcResponse
			var shutdownResp *rpcResponse
			for _, msg := range batch {
				resp, shutdown := s.handleBatchMessage(context.Background(), runtime, msg)
				if resp != nil {
					responses = append(responses, *resp)
				}
				if shutdown {
					shutdownResp = resp
					break
				}
			}
			if len(responses) > 0 {
				runtime.writeMu.Lock()
				err := runtime.encoder.Encode(responses)
				runtime.writeMu.Unlock()
				recordErr(err)
			}
			if shutdownResp != nil {
				wg.Wait()
				if err := getErr(); err != nil {
					return err
				}
				if shutdownResp.Error != nil {
					return shutdownResp.Error
				}
				return nil
			}
			continue
		}
		var msg rpcMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			return err
		}
		resp, shutdown := s.handleInboundMessage(context.Background(), runtime, &wg, recordErr, msg)
		if resp != nil {
			runtime.writeMu.Lock()
			err := runtime.encoder.Encode(resp)
			runtime.writeMu.Unlock()
			recordErr(err)
		}
		if shutdown {
			wg.Wait()
			if err := getErr(); err != nil {
				return err
			}
			if resp != nil && resp.Error != nil {
				return resp.Error
			}
			return nil
		}
	}
}

func (s *Server) handleBatchMessage(ctx context.Context, runtime *serverRuntime, msg rpcMessage) (*rpcResponse, bool) {
	if msg.Method == "" {
		runtime.deliverResponse(rpcResponse{
			JSONRPC: msg.JSONRPC,
			ID:      msg.ID,
			Result:  msg.Result,
			Error:   msg.Error,
		})
		return nil, false
	}
	req := rpcRequest{JSONRPC: msg.JSONRPC, ID: msg.ID, Method: msg.Method, Params: msg.Params}
	ctx = context.WithValue(ctx, callbackRuntimeKey{}, runtime)
	resp := s.handleRequest(ctx, req)
	return &resp, req.Method == "plugin.shutdown"
}

func (s *Server) handleInboundMessage(ctx context.Context, runtime *serverRuntime, wg *sync.WaitGroup, recordErr func(error), msg rpcMessage) (*rpcResponse, bool) {
	if msg.Method == "" {
		runtime.deliverResponse(rpcResponse{
			JSONRPC: msg.JSONRPC,
			ID:      msg.ID,
			Result:  msg.Result,
			Error:   msg.Error,
		})
		return nil, false
	}
	req := rpcRequest{JSONRPC: msg.JSONRPC, ID: msg.ID, Method: msg.Method, Params: msg.Params}
	if req.Method == "plugin.shutdown" {
		resp := s.handleRequest(ctx, req)
		return &resp, true
	}
	wg.Add(1)
	go func(req rpcRequest) {
		defer wg.Done()
		ctx := context.WithValue(context.Background(), callbackRuntimeKey{}, runtime)
		resp := s.handleRequest(ctx, req)
		runtime.writeMu.Lock()
		err := runtime.encoder.Encode(resp)
		runtime.writeMu.Unlock()
		recordErr(err)
	}(req)
	return nil, false
}

func (r *serverRuntime) callCallback(ctx context.Context, params callbackCallParams) (Value, error) {
	id := r.nextID.Add(1)
	ch := make(chan rpcResponse, 1)
	r.mu.Lock()
	r.pending[id] = ch
	r.mu.Unlock()

	req := rpcRequest{JSONRPC: "2.0", ID: id, Method: "callback.call", Params: params}
	r.writeMu.Lock()
	err := r.encoder.Encode(req)
	r.writeMu.Unlock()
	if err != nil {
		r.removePending(id)
		return Value{}, err
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return Value{}, resp.Error
		}
		if len(resp.Result) == 0 {
			return Value{Type: valueNull}, nil
		}
		var result Value
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			return Value{}, err
		}
		return result, nil
	case <-ctx.Done():
		r.removePending(id)
		return Value{}, ctx.Err()
	}
}

func (r *serverRuntime) deliverResponse(resp rpcResponse) {
	r.mu.Lock()
	ch := r.pending[resp.ID]
	delete(r.pending, resp.ID)
	r.mu.Unlock()
	if ch != nil {
		ch <- resp
	}
}

func (r *serverRuntime) removePending(id int64) {
	r.mu.Lock()
	delete(r.pending, id)
	r.mu.Unlock()
}

func (s *Server) handleRequest(ctx context.Context, req rpcRequest) rpcResponse {
	result, err := s.dispatch(ctx, req.Method, req.Params)
	resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	if err != nil {
		resp.Error = &RPCError{Code: -32000, Message: err.Error()}
		return resp
	}
	if result != nil {
		raw, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			resp.Error = &RPCError{Code: -32000, Message: marshalErr.Error()}
			return resp
		}
		resp.Result = raw
	}
	return resp
}

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
	case "environment.open", "environment.close", "plugin.shutdown":
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
	instance := &object.Instance{Class: class, Fields: make(map[string]object.Object)}
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
		result := evaluator.ApplyFunction(ctx, init, callArgs, objKwargs, object.NewEnvironment())
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
	result := evaluator.ApplyFunction(ctx, methodObj, callArgs, objKwargs, object.NewEnvironment())
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
	result := evaluator.ApplyFunction(ctx, target, callArgs, objKwargs, object.NewEnvironment())
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
	if del, exists := remoteObject.class.LookupMember("__del__"); exists {
		evaluator.ApplyFunction(context.Background(), del, []object.Object{remoteObject.instance}, nil, object.NewEnvironment())
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
