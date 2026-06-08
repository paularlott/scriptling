package plugin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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
	functions   map[string]*funcEntry
	classes     map[string]*classEntry
	constants   map[string]Value
}

type funcEntry struct {
	builtin *object.Builtin
	source  string
}

type classEntry struct {
	class  *object.Class
	source string
}

type serverClass struct {
	name        string
	objectClass *object.Class
	objects     map[string]*object.Instance
	nextID      atomic.Int64
	mu          sync.Mutex
}

func NewServer(name, version, description string) *Server {
	return &Server{
		name:        declaredLibraryName(name),
		version:     version,
		description: description,
		functions:   make(map[string]*funcEntry),
		classes:     make(map[string]*classEntry),
		constants:   make(map[string]Value),
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
	encoder := json.NewEncoder(output)
	var writeMu sync.Mutex
	var wg sync.WaitGroup

	for {
		var req rpcRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				wg.Wait()
				return nil
			}
			return err
		}
		if req.Method == "plugin.shutdown" {
			resp := s.handleRequest(req)
			writeMu.Lock()
			err := encoder.Encode(resp)
			writeMu.Unlock()
			wg.Wait()
			return err
		}
		wg.Add(1)
		go func(req rpcRequest) {
			defer wg.Done()
			resp := s.handleRequest(req)
			writeMu.Lock()
			encoder.Encode(resp)
			writeMu.Unlock()
		}(req)
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
			for mname := range entry.class.Methods {
				if mname == "__init__" || mname == "__del__" {
					continue
				}
				methods = append(methods, FunctionSchema{Name: mname})
			}
			cs.Methods = methods
		}
		classes = append(classes, cs)
	}
	constants := make([]ConstantSchema, 0, len(s.constants))
	for name, value := range s.constants {
		constants = append(constants, ConstantSchema{Name: name, Value: value})
	}
	return Schema{Functions: functions, Classes: classes, Constants: constants}
}

func (s *Server) callFunction(params functionCallParams) (Value, error) {
	entry, ok := s.functions[params.Name]
	if !ok || entry.builtin == nil {
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
	result := entry.builtin.Fn(context.Background(), object.NewKwargs(kwargs), args...)
	return objectToValue(result)
}

func (s *Server) newObject(params objectNewParams) (*RemoteRef, error) {
	entry, ok := s.classes[params.Class]
	if !ok || entry.class == nil {
		return nil, fmt.Errorf("unknown class %s", params.Class)
	}
	class := entry.class
	sc := &serverClass{
		name:        class.Name,
		objectClass: class,
		objects:     make(map[string]*object.Instance),
	}
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
		result := evaluator.ApplyFunction(context.Background(), init, callArgs, objKwargs, object.NewEnvironment())
		if errObj, ok := result.(*object.Error); ok {
			return nil, errors.New(errObj.Message)
		}
	}
	id := strconv.FormatInt(sc.nextID.Add(1), 10)
	sc.objects[id] = instance
	s.storeClass(sc)
	return &RemoteRef{
		Library: s.name,
		Class:   class.Name,
		ID:      id,
	}, nil
}

func (s *Server) callMethod(params methodCallParams) (Value, error) {
	sc := s.loadClass(params.ObjectID)
	if sc == nil {
		return Value{}, fmt.Errorf("unknown object %s", params.ObjectID)
	}
	sc.mu.Lock()
	instance, ok := sc.objects[params.ObjectID]
	sc.mu.Unlock()
	if !ok {
		return Value{}, fmt.Errorf("unknown object %s", params.ObjectID)
	}
	methodObj, ok := sc.objectClass.LookupMember(params.Method)
	if !ok {
		return Value{}, fmt.Errorf("unknown method %s", params.Method)
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
	result := evaluator.ApplyFunction(context.Background(), methodObj, callArgs, objKwargs, object.NewEnvironment())
	if errObj, ok := result.(*object.Error); ok {
		return Value{}, errors.New(errObj.Message)
	}
	return objectToValue(result)
}

func (s *Server) destroyObject(params objectDestroyParams) error {
	sc := s.loadClass(params.ObjectID)
	if sc == nil {
		return nil
	}
	sc.mu.Lock()
	instance, ok := sc.objects[params.ObjectID]
	if ok {
		delete(sc.objects, params.ObjectID)
	}
	sc.mu.Unlock()
	if !ok {
		return nil
	}
	if del, exists := sc.objectClass.LookupMember("__del__"); exists {
		evaluator.ApplyFunction(context.Background(), del, []object.Object{instance}, nil, object.NewEnvironment())
	}
	return nil
}

var serverClasses sync.Map

type classRef struct {
	sc *serverClass
}

func (s *Server) storeClass(sc *serverClass) {
	serverClasses.Store(sc, &classRef{sc: sc})
}

func (s *Server) loadClass(objectID string) *serverClass {
	var found *serverClass
	serverClasses.Range(func(key, value any) bool {
		sc := key.(*serverClass)
		sc.mu.Lock()
		_, ok := sc.objects[objectID]
		sc.mu.Unlock()
		if ok {
			found = sc
			return false
		}
		return true
	})
	return found
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
