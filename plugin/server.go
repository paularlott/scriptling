package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/object"
)

type Server struct {
	name        string
	version     string
	description string
	functions   map[string]*object.Builtin
	classes     map[string]*serverClass
	constants   map[string]Value
	wrappers    []WrapperSchema
}

type serverClass struct {
	name        string
	constructor any
	methods     map[string]any
	destructor  any
	objectClass *object.Class
	objects     map[string]any
	nextID      atomic.Int64
	mu          sync.Mutex
}

type ClassServerBuilder struct {
	server *Server
	class  *serverClass
}

func NewServer(name, version, description string) *Server {
	return &Server{
		name:        declaredLibraryName(name),
		version:     version,
		description: description,
		functions:   make(map[string]*object.Builtin),
		classes:     make(map[string]*serverClass),
		constants:   make(map[string]Value),
	}
}

func (s *Server) Function(name string, fn any) *Server {
	builder := object.NewFunctionBuilder()
	builder.Function(fn)
	s.functions[name] = &object.Builtin{Fn: builder.Build()}
	return s
}

func (s *Server) HiddenFunction(name string, fn any) *Server {
	builder := object.NewFunctionBuilder()
	builder.Function(fn)
	s.functions[name] = &object.Builtin{Fn: builder.Build(), Attributes: map[string]object.Object{
		"__plugin_hidden__": object.NewBoolean(true),
	}}
	return s
}

func (s *Server) FunctionBuiltin(name string, fn object.BuiltinFunction) *Server {
	s.functions[name] = &object.Builtin{Fn: fn}
	return s
}

func (s *Server) RegisterFunc(name string, fn object.BuiltinFunction) *Server {
	return s.FunctionBuiltin(name, fn)
}

func (s *Server) FunctionFromBuilder(name string, builder *object.FunctionBuilder) *Server {
	s.functions[name] = &object.Builtin{Fn: builder.Build()}
	return s
}

func (s *Server) Constant(name string, value any) *Server {
	s.constants[name] = goValueToTransport(value)
	return s
}

func (s *Server) Wrapper(name, source string) *Server {
	s.wrappers = append(s.wrappers, WrapperSchema{Name: name, Source: source})
	return s
}

func (s *Server) Class(name string) *ClassServerBuilder {
	class := &serverClass{
		name:    name,
		methods: make(map[string]any),
		objects: make(map[string]any),
	}
	s.classes[name] = class
	return &ClassServerBuilder{server: s, class: class}
}

func (s *Server) ClassFromBuilder(class *object.Class) *Server {
	return s.RegisterClass(class)
}

func (s *Server) RegisterClass(class *object.Class) *Server {
	serverClass := &serverClass{
		name:        class.Name,
		objectClass: class,
		methods:     make(map[string]any),
		objects:     make(map[string]any),
	}
	for name := range class.Methods {
		if name != "__init__" {
			serverClass.methods[name] = nil
		}
	}
	s.classes[class.Name] = serverClass
	return s
}

func (b *ClassServerBuilder) Constructor(fn any) *ClassServerBuilder {
	b.class.constructor = fn
	return b
}

func (b *ClassServerBuilder) Method(name string, fn any) *ClassServerBuilder {
	b.class.methods[name] = fn
	return b
}

func (b *ClassServerBuilder) Destructor(fn any) *ClassServerBuilder {
	b.class.destructor = fn
	return b
}

func (s *Server) Run() error {
	return s.RunIO(os.Stdin, os.Stdout)
}

func (s *Server) RunIO(input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(bufio.NewReader(input))
	encoder := json.NewEncoder(output)
	writeMu := sync.Mutex{}

	for {
		var req rpcRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		resp := s.handleRequest(req)
		writeMu.Lock()
		err := encoder.Encode(resp)
		writeMu.Unlock()
		if err != nil {
			return err
		}
		if req.Method == "plugin.shutdown" {
			return nil
		}
	}
}

func (s *Server) handleRequest(req rpcRequest) rpcResponse {
	result, err := s.dispatch(req.Method, req.Params)
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

func (s *Server) dispatch(method string, params any) (any, error) {
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
		return s.callFunction(p)
	case "object.new":
		var p objectNewParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		return s.newObject(p)
	case "object.call_method":
		var p methodCallParams
		if err := decodeParams(params, &p); err != nil {
			return nil, err
		}
		return s.callMethod(p)
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
	for name := range s.functions {
		hidden := false
		if attrs := s.functions[name].Attributes; attrs != nil {
			if value, ok := attrs["__plugin_hidden__"].(*object.Boolean); ok {
				hidden, _ = value.AsBool()
			}
		}
		functions = append(functions, FunctionSchema{Name: name, Wrapper: "generated", Hidden: hidden})
	}
	classes := make([]ClassSchema, 0, len(s.classes))
	for _, class := range s.classes {
		methods := make([]FunctionSchema, 0, len(class.methods))
		for name := range class.methods {
			methods = append(methods, FunctionSchema{Name: name, Wrapper: "generated"})
		}
		classes = append(classes, ClassSchema{
			Name:        class.name,
			Constructor: FunctionSchema{Name: class.name, Wrapper: "generated"},
			Methods:     methods,
		})
	}
	constants := make([]ConstantSchema, 0, len(s.constants))
	for name, value := range s.constants {
		constants = append(constants, ConstantSchema{Name: name, Value: value})
	}
	return Schema{Functions: functions, Classes: classes, Constants: constants, Wrappers: s.wrappers}
}

func (s *Server) callFunction(params functionCallParams) (Value, error) {
	fn, ok := s.functions[params.Name]
	if !ok {
		return Value{}, fmt.Errorf("unknown function %s", params.Name)
	}
	args, err := transportValuesToObjects(params.Args)
	if err != nil {
		return Value{}, err
	}
	kwargs, err := transportKwargsToObjects(params.Kwargs)
	if err != nil {
		return Value{}, err
	}
	result := fn.Fn(context.Background(), object.NewKwargs(kwargs), args...)
	return objectToValue(result)
}

func (s *Server) newObject(params objectNewParams) (*RemoteRef, error) {
	class, ok := s.classes[params.Class]
	if !ok {
		return nil, fmt.Errorf("unknown class %s", params.Class)
	}
	if class.constructor == nil && class.objectClass == nil {
		return nil, fmt.Errorf("class %s has no constructor", params.Class)
	}
	instance, err := class.newInstance(params.Args, params.Kwargs)
	if err != nil {
		return nil, err
	}
	id := strconv.FormatInt(class.nextID.Add(1), 10)
	class.mu.Lock()
	class.objects[id] = instance
	class.mu.Unlock()
	return &RemoteRef{
		Library:       s.name,
		Class:         class.name,
		EnvironmentID: params.EnvironmentID,
		ID:            id,
	}, nil
}

func (s *Server) callMethod(params methodCallParams) (Value, error) {
	for _, class := range s.classes {
		class.mu.Lock()
		instance, ok := class.objects[params.ObjectID]
		class.mu.Unlock()
		if !ok {
			continue
		}
		method, ok := class.methods[params.Method]
		if !ok {
			return Value{}, fmt.Errorf("unknown method %s", params.Method)
		}
		result, err := class.callMethod(instance, method, params)
		if err != nil {
			return Value{}, err
		}
		return goValueToTransport(result), nil
	}
	return Value{}, fmt.Errorf("unknown object %s", params.ObjectID)
}

func (c *serverClass) newInstance(args []Value, kwargs map[string]Value) (any, error) {
	if c.objectClass != nil {
		instance := &object.Instance{Class: c.objectClass, Fields: make(map[string]object.Object)}
		if init, ok := c.objectClass.LookupMember("__init__"); ok {
			objArgs, err := transportValuesToObjects(args)
			if err != nil {
				return nil, err
			}
			objKwargs, err := transportKwargsToObjects(kwargs)
			if err != nil {
				return nil, err
			}
			callArgs := append([]object.Object{instance}, objArgs...)
			result := evaluator.ApplyFunction(context.Background(), init, callArgs, objKwargs, object.NewEnvironment())
			if errObj, ok := result.(*object.Error); ok {
				return nil, errors.New(errObj.Message)
			}
		}
		return instance, nil
	}
	anyArgs, err := transportValuesToAny(args)
	if err != nil {
		return nil, err
	}
	return callReflect(c.constructor, nil, anyArgs)
}

func (c *serverClass) callMethod(instance any, method any, params methodCallParams) (any, error) {
	if c.objectClass != nil {
		methodObj, ok := c.objectClass.LookupMember(params.Method)
		if !ok {
			return nil, fmt.Errorf("unknown method %s", params.Method)
		}
		objInstance, ok := instance.(*object.Instance)
		if !ok {
			return nil, fmt.Errorf("stored object is not a Scriptling instance")
		}
		objArgs, err := transportValuesToObjects(params.Args)
		if err != nil {
			return nil, err
		}
		objKwargs, err := transportKwargsToObjects(params.Kwargs)
		if err != nil {
			return nil, err
		}
		callArgs := append([]object.Object{objInstance}, objArgs...)
		result := evaluator.ApplyFunction(context.Background(), methodObj, callArgs, objKwargs, object.NewEnvironment())
		if errObj, ok := result.(*object.Error); ok {
			return nil, errors.New(errObj.Message)
		}
		return result, nil
	}
	anyArgs, err := transportValuesToAny(params.Args)
	if err != nil {
		return nil, err
	}
	return callReflect(method, instance, anyArgs)
}

func (s *Server) destroyObject(params objectDestroyParams) error {
	for _, class := range s.classes {
		class.mu.Lock()
		instance, ok := class.objects[params.ObjectID]
		if ok {
			delete(class.objects, params.ObjectID)
		}
		class.mu.Unlock()
		if !ok {
			continue
		}
		if class.destructor != nil {
			_, err := callReflect(class.destructor, instance, nil)
			return err
		}
		return nil
	}
	return nil
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

func transportValuesToAny(values []Value) ([]any, error) {
	args := make([]any, 0, len(values))
	for _, value := range values {
		decoded, err := transportValueToAny(value)
		if err != nil {
			return nil, err
		}
		args = append(args, decoded)
	}
	return args, nil
}

func transportValueToAny(value Value) (any, error) {
	switch value.Type {
	case "", valueNull:
		return nil, nil
	case valueBool, valueInt, valueFloat, valueString:
		return value.Value, nil
	case valueList:
		items := make([]any, 0, len(value.Items))
		for _, item := range value.Items {
			decoded, err := transportValueToAny(item)
			if err != nil {
				return nil, err
			}
			items = append(items, decoded)
		}
		return items, nil
	case valueDict:
		entries := make(map[string]any, len(value.Entries))
		for key, item := range value.Entries {
			decoded, err := transportValueToAny(item)
			if err != nil {
				return nil, err
			}
			entries[key] = decoded
		}
		return entries, nil
	default:
		return nil, fmt.Errorf("unsupported transport value %q", value.Type)
	}
}

func goValueToTransport(value any) Value {
	switch v := value.(type) {
	case nil:
		return Value{Type: valueNull}
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
		return Value{Type: valueString, Value: fmt.Sprint(value)}
	}
}

func callReflect(fn any, receiver any, args []any) (any, error) {
	value := reflect.ValueOf(fn)
	if value.Kind() != reflect.Func {
		return nil, fmt.Errorf("registered value is not a function")
	}
	fnType := value.Type()
	values := make([]reflect.Value, 0, fnType.NumIn())
	offset := 0
	if receiver != nil {
		values = append(values, reflect.ValueOf(receiver))
		offset = 1
	}
	if len(args)+offset != fnType.NumIn() {
		return nil, fmt.Errorf("expected %d args, got %d", fnType.NumIn()-offset, len(args))
	}
	for i, arg := range args {
		targetType := fnType.In(i + offset)
		converted, err := convertReflectArg(arg, targetType)
		if err != nil {
			return nil, err
		}
		values = append(values, converted)
	}
	results := value.Call(values)
	if len(results) == 0 {
		return nil, nil
	}
	if len(results) == 2 {
		if errValue := results[1]; !errValue.IsNil() {
			if err, ok := errValue.Interface().(error); ok {
				return nil, err
			}
		}
	}
	return results[0].Interface(), nil
}

func convertReflectArg(arg any, targetType reflect.Type) (reflect.Value, error) {
	if arg == nil {
		return reflect.Zero(targetType), nil
	}
	value := reflect.ValueOf(arg)
	if value.Type().AssignableTo(targetType) {
		return value, nil
	}
	if value.Type().ConvertibleTo(targetType) {
		return value.Convert(targetType), nil
	}
	switch targetType.Kind() {
	case reflect.Int:
		return reflect.ValueOf(int(numberToInt64(arg))), nil
	case reflect.Int64:
		return reflect.ValueOf(numberToInt64(arg)), nil
	case reflect.Float64:
		return reflect.ValueOf(numberToFloat64(arg)), nil
	case reflect.String:
		if s, ok := arg.(string); ok {
			return reflect.ValueOf(s), nil
		}
	case reflect.Map:
		if targetType.Key().Kind() == reflect.String && targetType.Elem().Kind() == reflect.Interface {
			if m, ok := arg.(map[string]any); ok {
				out := reflect.MakeMap(targetType)
				for key, item := range m {
					out.SetMapIndex(reflect.ValueOf(key), reflect.ValueOf(item))
				}
				return out, nil
			}
		}
	}
	return reflect.Value{}, fmt.Errorf("cannot convert %T to %s", arg, targetType)
}
