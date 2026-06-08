package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func resetGlobals(t *testing.T) {
	t.Helper()
	t.Cleanup(func() { serverClasses.Clear() })
}

func TestServerFunctionCall(t *testing.T) {
	resetGlobals(t)
	fb := object.NewFunctionBuilder()
	fb.Function(func(a int, b int) int {
		return a + b
	})

	server := NewServer("mathy", "1.0.0", "test math").
		RegisterFunc("add", fb)

	result := sendServerRequest[Value](t, server, "function.call", functionCallParams{
		Name: "add",
		Args: []Value{
			{Type: valueInt, Value: int64(2)},
			{Type: valueInt, Value: int64(3)},
		},
	})

	if result.Type != valueInt || numberToInt64(result.Value) != 5 {
		t.Fatalf("expected int 5, got %#v", result)
	}
}

func TestServerClassLifecycle(t *testing.T) {
	resetGlobals(t)
	destroyed := false
	class := object.NewClassBuilder("Config").
		Method("__init__", func(self *object.Instance, name string) {
			self.Fields["name"] = object.NewString(name)
		}).
		Method("get", func(self *object.Instance) string {
			return self.Fields["name"].(*object.String).StringValue()
		}).
		Method("__del__", func(self *object.Instance) {
			destroyed = true
		})

	server := NewServer("config", "1.0.0", "test config")
	server.RegisterClass(class)

	ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{
		Class: "Config",
		Args:  []Value{{Type: valueString, Value: "scriptling"}},
	})

	if ref.ID == "" || ref.Class != "Config" || ref.Library != "config" {
		t.Fatalf("unexpected ref: %#v", ref)
	}

	got := sendServerRequest[Value](t, server, "object.call_method", methodCallParams{
		ObjectID: ref.ID,
		Method:   "get",
	})
	if got.Type != valueString || got.Value != "scriptling" {
		t.Fatalf("expected string result, got %#v", got)
	}

	_ = sendServerRequest[any](t, server, "object.destroy", objectDestroyParams{
		ObjectID: ref.ID,
	})
	if !destroyed {
		t.Fatal("expected __del__ to run")
	}
}

func TestServerHandshakeSchema(t *testing.T) {
	resetGlobals(t)
	fb := object.NewFunctionBuilder()
	fb.Function(func(name string) string { return "hello " + name })

	server := NewServer("hello", "1.2.3", "hello plugin").
		RegisterFunc("greet", fb).
		Constant("answer", 42)

	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{
		Protocol:   ProtocolVersion,
		Transports: []string{"json"},
	})

	if result.Protocol != ProtocolVersion || result.Transport != "json" {
		t.Fatalf("unexpected handshake result: %#v", result)
	}
	if result.Library.Name != "hello" || result.Library.Version != "1.2.3" {
		t.Fatalf("unexpected library metadata: %#v", result.Library)
	}
	if len(result.Schema.Functions) != 1 || result.Schema.Functions[0].Name != "greet" {
		t.Fatalf("unexpected functions schema: %#v", result.Schema.Functions)
	}
	if len(result.Schema.Constants) != 1 || result.Schema.Constants[0].Name != "answer" {
		t.Fatalf("unexpected constants schema: %#v", result.Schema.Constants)
	}
}

func TestServerRegisterClass(t *testing.T) {
	resetGlobals(t)
	class := object.NewClassBuilder("Counter").
		Method("__init__", func(self *object.Instance, start int) {
			self.Fields["value"] = object.NewInteger(int64(start))
		}).
		Method("inc", func(self *object.Instance, amount int) int {
			current := self.Fields["value"].(*object.Integer).IntValue()
			next := current + int64(amount)
			self.Fields["value"] = object.NewInteger(next)
			return int(next)
		}).
		Method("get", func(self *object.Instance) int {
			return int(self.Fields["value"].(*object.Integer).IntValue())
		})

	server := NewServer("builderclass", "1.0.0", "builder class").
		RegisterClass(class)

	ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{
		Class: "Counter",
		Args:  []Value{{Type: valueInt, Value: int64(4)}},
	})

	got := sendServerRequest[Value](t, server, "object.call_method", methodCallParams{
		ObjectID: ref.ID,
		Method:   "inc",
		Args:     []Value{{Type: valueInt, Value: int64(3)}},
	})
	if got.Type != valueInt || numberToInt64(got.Value) != 7 {
		t.Fatalf("expected counter value 7, got %#v", got)
	}

	got = sendServerRequest[Value](t, server, "object.call_method", methodCallParams{
		ObjectID: ref.ID,
		Method:   "get",
	})
	if got.Type != valueInt || numberToInt64(got.Value) != 7 {
		t.Fatalf("expected counter get 7, got %#v", got)
	}
}

func TestServerTypedReceiverClass(t *testing.T) {
	resetGlobals(t)
	type serverCfg struct {
		values map[string]string
	}

	destroyed := false

	class := object.NewClassBuilder("ServerConfig").
		Constructor(func(name string) *serverCfg {
			return &serverCfg{values: map[string]string{"name": name}}
		}).
		Method("get", func(self *serverCfg, key string) string {
			return self.values[key]
		}).
		Method("set", func(self *serverCfg, key, val string) {
			self.values[key] = val
		}).
		Method("__del__", func(self *serverCfg) {
			destroyed = true
		})

	server := NewServer("typedcfg", "1.0.0", "typed config").
		RegisterClass(class)

	ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{
		Class: "ServerConfig",
		Args:  []Value{{Type: valueString, Value: "production"}},
	})

	got := sendServerRequest[Value](t, server, "object.call_method", methodCallParams{
		ObjectID: ref.ID,
		Method:   "get",
		Args:     []Value{{Type: valueString, Value: "name"}},
	})
	if got.Type != valueString || got.Value != "production" {
		t.Fatalf("expected 'production', got %#v", got)
	}

	_ = sendServerRequest[any](t, server, "object.call_method", methodCallParams{
		ObjectID: ref.ID,
		Method:   "set",
		Args:     []Value{{Type: valueString, Value: "port"}, {Type: valueString, Value: "8080"}},
	})

	got = sendServerRequest[Value](t, server, "object.call_method", methodCallParams{
		ObjectID: ref.ID,
		Method:   "get",
		Args:     []Value{{Type: valueString, Value: "port"}},
	})
	if got.Type != valueString || got.Value != "8080" {
		t.Fatalf("expected '8080', got %#v", got)
	}

	_ = sendServerRequest[any](t, server, "object.destroy", objectDestroyParams{
		ObjectID: ref.ID,
	})
	if !destroyed {
		t.Fatal("expected __del__ to run")
	}
}

func TestServerNativeObjectAPI(t *testing.T) {
	resetGlobals(t)
	fb := object.NewFunctionBuilder()
	fb.FunctionWithHelp(func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		name, err := args[0].AsString()
		if err != nil {
			return err
		}
		return object.NewString("native:" + name)
	}, "upper_label(name)")

	server := NewServer("native", "1.0.0", "native object API").
		RegisterFunc("upper_label", fb)

	result := sendServerRequest[Value](t, server, "function.call", functionCallParams{
		Name: "upper_label",
		Args: []Value{{Type: valueString, Value: "Ada"}},
	})

	if result.Type != valueString || result.Value != "native:Ada" {
		t.Fatalf("expected native function result, got %#v", result)
	}
}

func TestServerEmbeddedScriptlingFunction(t *testing.T) {
	resetGlobals(t)
	p := scriptling.New()
	if err := p.RegisterScriptFunc("decorate", `
def decorate(name):
    return "[" + name + "]"
`); err != nil {
		t.Fatalf("RegisterScriptFunc: %v", err)
	}

	fb := object.NewFunctionBuilder()
	fb.Function(func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		name, err := args[0].AsString()
		if err != nil {
			return err
		}
		result, callErr := p.CallFunction("decorate", name)
		if callErr != nil {
			return object.NewString(callErr.Error())
		}
		return result
	})

	server := NewServer("embedded", "1.0.0", "embedded scriptling").
		RegisterFunc("decorate", fb)

	result := sendServerRequest[Value](t, server, "function.call", functionCallParams{
		Name: "decorate",
		Args: []Value{{Type: valueString, Value: "Ada"}},
	})

	if result.Type != valueString || result.Value != "[Ada]" {
		t.Fatalf("expected embedded Scriptling result, got %#v", result)
	}
}

func TestServerWrapperMode(t *testing.T) {
	resetGlobals(t)
	fb := object.NewFunctionBuilder()
	fb.Function(func(name string) string {
		return "Hello, " + name
	})

	server := NewServer("wrap", "1.0.0", "wrapper test").
		RegisterFunc("greet", fb).
		Wrapper("greet", `
def greet(name):
    return "wrapped:" + name
`)

	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{
		Protocol:   ProtocolVersion,
		Transports: []string{"json"},
	})

	if len(result.Schema.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(result.Schema.Functions))
	}
	fn := result.Schema.Functions[0]
	if fn.Name != "greet" || fn.Mode != ModeWrapper {
		t.Fatalf("expected wrapper mode, got name=%s mode=%s", fn.Name, fn.Mode)
	}
	if fn.Source == "" {
		t.Fatal("expected wrapper source in schema")
	}
}

func TestServerScriptFunction(t *testing.T) {
	resetGlobals(t)
	server := NewServer("scripted", "1.0.0", "script function test").
		RegisterScriptFunc("helper", `
def helper(x):
    return x * 2
`)

	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{
		Protocol:   ProtocolVersion,
		Transports: []string{"json"},
	})

	if len(result.Schema.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(result.Schema.Functions))
	}
	fn := result.Schema.Functions[0]
	if fn.Name != "helper" || fn.Mode != ModeScript {
		t.Fatalf("expected script mode, got name=%s mode=%s", fn.Name, fn.Mode)
	}
}

func TestServerScriptClass(t *testing.T) {
	resetGlobals(t)
	server := NewServer("scripted", "1.0.0", "script class test").
		RegisterScriptClass("Pair", `
class Pair:
    def __init__(self, a, b):
        self.a = a
        self.b = b
`)

	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{
		Protocol:   ProtocolVersion,
		Transports: []string{"json"},
	})

	if len(result.Schema.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(result.Schema.Classes))
	}
	cls := result.Schema.Classes[0]
	if cls.Name != "Pair" || cls.Mode != ModeScript {
		t.Fatalf("expected script mode, got name=%s mode=%s", cls.Name, cls.Mode)
	}
}

func sendServerRequest[T any](t *testing.T, server *Server, method string, params any) T {
	t.Helper()

	var input bytes.Buffer
	var output bytes.Buffer
	encoder := json.NewEncoder(&input)
	if err := encoder.Encode(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params}); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	if err := server.RunIO(&input, &output); err != nil {
		t.Fatalf("RunIO: %v", err)
	}
	var resp rpcResponse
	if err := json.NewDecoder(&output).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("response error: %v", resp.Error)
	}
	var result T
	if len(resp.Result) > 0 {
		if err := json.Unmarshal(resp.Result, &result); err != nil {
			t.Fatalf("decode result: %v", err)
		}
	}
	return result
}

func sendServerRequestExpectError(t *testing.T, server *Server, method string, params any) *RPCError {
	t.Helper()
	var input bytes.Buffer
	var output bytes.Buffer
	encoder := json.NewEncoder(&input)
	if err := encoder.Encode(rpcRequest{JSONRPC: "2.0", ID: 1, Method: method, Params: params}); err != nil {
		t.Fatalf("encode request: %v", err)
	}
	if err := server.RunIO(&input, &output); err != nil {
		t.Fatalf("RunIO: %v", err)
	}
	var resp rpcResponse
	if err := json.NewDecoder(&output).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.Error
}

func TestServerDataTypeRoundtrips(t *testing.T) {
	resetGlobals(t)
	echoInt := object.NewFunctionBuilder()
	echoInt.Function(func(v int) int { return v })
	echoFloat := object.NewFunctionBuilder()
	echoFloat.Function(func(v float64) float64 { return v })
	echoString := object.NewFunctionBuilder()
	echoString.Function(func(v string) string { return v })
	echoBool := object.NewFunctionBuilder()
	echoBool.Function(func(v bool) bool { return v })
	echoAny := object.NewFunctionBuilder()
	echoAny.Function(func(v any) any { return v })

	server := NewServer("echo", "1.0.0", "echo plugin").
		RegisterFunc("echo_int", echoInt).
		RegisterFunc("echo_float", echoFloat).
		RegisterFunc("echo_string", echoString).
		RegisterFunc("echo_bool", echoBool).
		RegisterFunc("echo_any", echoAny)

	tests := []struct {
		name   string
		fnName string
		arg    Value
		check  func(t *testing.T, result Value)
	}{
		{"bool", "echo_bool", Value{Type: valueBool, Value: true}, func(t *testing.T, r Value) {
			if r.Type != valueBool || r.Value != true {
				t.Errorf("expected bool true, got %+v", r)
			}
		}},
		{"int", "echo_int", Value{Type: valueInt, Value: int64(42)}, func(t *testing.T, r Value) {
			if r.Type != valueInt || numberToInt64(r.Value) != 42 {
				t.Errorf("expected int 42, got %+v", r)
			}
		}},
		{"float", "echo_float", Value{Type: valueFloat, Value: 3.14}, func(t *testing.T, r Value) {
			if r.Type != valueFloat || numberToFloat64(r.Value) != 3.14 {
				t.Errorf("expected float 3.14, got %+v", r)
			}
		}},
		{"string", "echo_string", Value{Type: valueString, Value: "hello"}, func(t *testing.T, r Value) {
			if r.Type != valueString || r.Value != "hello" {
				t.Errorf("expected string hello, got %+v", r)
			}
		}},
		{"list via any", "echo_any", Value{Type: valueList, Items: []Value{
			{Type: valueInt, Value: int64(1)},
			{Type: valueString, Value: "two"},
		}}, func(t *testing.T, r Value) {
			if r.Type != valueList || len(r.Items) != 2 {
				t.Errorf("expected list with 2 items, got %+v", r)
			}
		}},
		{"dict via any", "echo_any", Value{Type: valueDict, Entries: map[string]Value{
			"key": {Type: valueString, Value: "val"},
		}}, func(t *testing.T, r Value) {
			if r.Type != valueDict || len(r.Entries) != 1 {
				t.Errorf("expected dict with 1 entry, got %+v", r)
			}
		}},
		{"nested via any", "echo_any", Value{Type: valueList, Items: []Value{
			{Type: valueDict, Entries: map[string]Value{
				"nums": {Type: valueList, Items: []Value{
					{Type: valueInt, Value: int64(1)},
					{Type: valueInt, Value: int64(2)},
				}},
			}},
		}}, func(t *testing.T, r Value) {
			if r.Type != valueList || len(r.Items) != 1 {
				t.Fatalf("expected list with 1 item, got %+v", r)
			}
			if r.Items[0].Type != valueDict {
				t.Errorf("expected nested dict, got %q", r.Items[0].Type)
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sendServerRequest[Value](t, server, "function.call", functionCallParams{
				Name: tt.fnName,
				Args: []Value{tt.arg},
			})
			tt.check(t, result)
		})
	}
}

func TestServerErrorPaths(t *testing.T) {
	resetGlobals(t)
	server := NewServer("test", "1.0.0", "test")

	t.Run("unknown function", func(t *testing.T) {
		rpcErr := sendServerRequestExpectError(t, server, "function.call", functionCallParams{
			Name: "nonexistent",
		})
		if rpcErr == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unknown class", func(t *testing.T) {
		rpcErr := sendServerRequestExpectError(t, server, "object.new", objectNewParams{
			Class: "Nonexistent",
		})
		if rpcErr == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("unknown rpc method", func(t *testing.T) {
		rpcErr := sendServerRequestExpectError(t, server, "bogus.method", nil)
		if rpcErr == nil {
			t.Fatal("expected error")
		}
	})
}

func TestServerErrorPathsOnObjects(t *testing.T) {
	resetGlobals(t)
	class := object.NewClassBuilder("Item").
		Method("__init__", func(self *object.Instance) {}).
		Method("get", func(self *object.Instance) string { return "ok" })

	server := NewServer("objtest", "1.0.0", "test").RegisterClass(class)

	ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{
		Class: "Item",
	})

	t.Run("unknown method", func(t *testing.T) {
		rpcErr := sendServerRequestExpectError(t, server, "object.call_method", methodCallParams{
			ObjectID: ref.ID,
			Method:   "nonexistent",
		})
		if rpcErr == nil {
			t.Fatal("expected error for unknown method")
		}
	})

	t.Run("unknown object id", func(t *testing.T) {
		rpcErr := sendServerRequestExpectError(t, server, "object.call_method", methodCallParams{
			ObjectID: "bogus-id",
			Method:   "get",
		})
		if rpcErr == nil {
			t.Fatal("expected error for unknown object")
		}
	})

	t.Run("destroy unknown object", func(t *testing.T) {
		rpcErr := sendServerRequestExpectError(t, server, "object.destroy", objectDestroyParams{
			ObjectID: "nonexistent",
		})
		if rpcErr != nil {
			t.Fatalf("destroy unknown should be no-op, got error: %v", rpcErr)
		}
	})
}

func TestServerClassWithoutInit(t *testing.T) {
	resetGlobals(t)
	class := object.NewClassBuilder("Plain").
		Method("describe", func(self *object.Instance) string { return "plain" })

	server := NewServer("plain", "1.0.0", "test").RegisterClass(class)

	ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{
		Class: "Plain",
	})
	if ref.ID == "" {
		t.Fatal("expected object to be created without __init__")
	}

	result := sendServerRequest[Value](t, server, "object.call_method", methodCallParams{
		ObjectID: ref.ID,
		Method:   "describe",
	})
	if result.Type != valueString || result.Value != "plain" {
		t.Fatalf("expected 'plain', got %#v", result)
	}
}

func TestServerConstructorError(t *testing.T) {
	resetGlobals(t)
	type fragile struct{}

	class := object.NewClassBuilder("Fragile").
		Constructor(func(shouldFail bool) (*fragile, error) {
			if shouldFail {
				return nil, fmt.Errorf("construction failed")
			}
			return &fragile{}, nil
		})

	server := NewServer("errtest", "1.0.0", "test").RegisterClass(class)

	t.Run("success", func(t *testing.T) {
		ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{
			Class: "Fragile",
			Args:  []Value{{Type: valueBool, Value: false}},
		})
		if ref.ID == "" {
			t.Fatal("expected object to be created")
		}
	})

	t.Run("failure", func(t *testing.T) {
		rpcErr := sendServerRequestExpectError(t, server, "object.new", objectNewParams{
			Class: "Fragile",
			Args:  []Value{{Type: valueBool, Value: true}},
		})
		if rpcErr == nil {
			t.Fatal("expected error from constructor")
		}
	})
}

func TestServerMethodError(t *testing.T) {
	resetGlobals(t)
	fb := object.NewFunctionBuilder()
	fb.Function(func(s string) (string, error) {
		if s == "fail" {
			return "", fmt.Errorf("method error")
		}
		return "ok:" + s, nil
	})

	server := NewServer("merr", "1.0.0", "test").RegisterFunc("maybe_fail", fb)

	t.Run("success", func(t *testing.T) {
		result := sendServerRequest[Value](t, server, "function.call", functionCallParams{
			Name: "maybe_fail",
			Args: []Value{{Type: valueString, Value: "good"}},
		})
		if result.Type != valueString || result.Value != "ok:good" {
			t.Fatalf("expected 'ok:good', got %#v", result)
		}
	})

	t.Run("failure", func(t *testing.T) {
		result := sendServerRequest[Value](t, server, "function.call", functionCallParams{
			Name: "maybe_fail",
			Args: []Value{{Type: valueString, Value: "fail"}},
		})
		if result.Type != valueString || result.Value != "method error" {
			t.Fatalf("expected error string 'method error', got %#v", result)
		}
	})
}

func TestServerSchemaCompleteness(t *testing.T) {
	resetGlobals(t)
	type res struct{ name string }

	class := object.NewClassBuilder("Resource").
		Constructor(func(name string) *res { return &res{name: name} }).
		Method("get", func(self *res) string { return self.name }).
		Method("__del__", func(self *res) {})

	fb := object.NewFunctionBuilder()
	fb.Function(func() string { return "hi" })

	server := NewServer("schema", "1.0.0", "test").
		RegisterFunc("greet", fb).
		RegisterClass(class).
		Constant("max", 100).
		Constant("label", "test")

	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{
		Protocol: ProtocolVersion, Transports: []string{"json"},
	})

	if len(result.Schema.Functions) != 1 {
		t.Fatalf("expected 1 function, got %d", len(result.Schema.Functions))
	}
	if result.Schema.Functions[0].Name != "greet" || result.Schema.Functions[0].Mode != ModeRPC {
		t.Fatalf("unexpected function schema: %+v", result.Schema.Functions[0])
	}

	if len(result.Schema.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(result.Schema.Classes))
	}
	cls := result.Schema.Classes[0]
	if cls.Name != "Resource" || cls.Mode != ModeRPC {
		t.Fatalf("unexpected class schema: %+v", cls)
	}
	if cls.Constructor.Name != "Resource" || cls.Constructor.Mode != ModeRPC {
		t.Fatalf("unexpected constructor schema: %+v", cls.Constructor)
	}
	methodNames := make(map[string]bool)
	for _, m := range cls.Methods {
		methodNames[m.Name] = true
	}
	if !methodNames["get"] {
		t.Error("expected 'get' in class methods")
	}
	if methodNames["__init__"] || methodNames["__del__"] {
		t.Error("__init__ and __del__ should not appear in schema methods")
	}

	if len(result.Schema.Constants) != 2 {
		t.Fatalf("expected 2 constants, got %d", len(result.Schema.Constants))
	}
}

func TestServerSchemaClassWrapper(t *testing.T) {
	resetGlobals(t)
	class := object.NewClassBuilder("W").
		Constructor(func() *struct{} { return &struct{}{} }).
		Method("do", func(self *struct{}) string { return "done" })

	server := NewServer("wrapcls", "1.0.0", "test").
		RegisterClass(class).
		Wrapper("W", `class W: pass`)

	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{
		Protocol: ProtocolVersion, Transports: []string{"json"},
	})
	if len(result.Schema.Classes) != 1 {
		t.Fatalf("expected 1 class, got %d", len(result.Schema.Classes))
	}
	cls := result.Schema.Classes[0]
	if cls.Mode != ModeWrapper {
		t.Errorf("expected wrapper mode, got %q", cls.Mode)
	}
	if cls.Source == "" {
		t.Error("expected wrapper source")
	}
}

func TestServerNamespacePrefix(t *testing.T) {
	resetGlobals(t)
	server := NewServer("plugin.mylib", "1.0.0", "test")
	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{
		Protocol: ProtocolVersion, Transports: []string{"json"},
	})
	if result.Library.Name != "mylib" {
		t.Errorf("expected 'mylib', got %q", result.Library.Name)
	}
}

func TestServerConstantsAllTypes(t *testing.T) {
	resetGlobals(t)
	fb := object.NewFunctionBuilder()
	fb.Function(func() string { return "noop" })

	server := NewServer("consts", "1.0.0", "test").
		RegisterFunc("noop", fb).
		Constant("nil_val", nil).
		Constant("bool_val", true).
		Constant("int_val", 42).
		Constant("float_val", 3.14).
		Constant("str_val", "hello").
		Constant("list_val", []any{1, "two", true}).
		Constant("dict_val", map[string]any{"key": "val"})

	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{
		Protocol: ProtocolVersion, Transports: []string{"json"},
	})
	if len(result.Schema.Constants) != 7 {
		t.Fatalf("expected 7 constants, got %d", len(result.Schema.Constants))
	}
}

func TestServerMultipleRequests(t *testing.T) {
	resetGlobals(t)
	fb := object.NewFunctionBuilder()
	fb.Function(func(x int) int { return x * 2 })

	server := NewServer("multi", "1.0.0", "test").RegisterFunc("double", fb)

	var input bytes.Buffer
	var output bytes.Buffer
	enc := json.NewEncoder(&input)

	enc.Encode(rpcRequest{JSONRPC: "2.0", ID: 1, Method: "function.call", Params: functionCallParams{
		Name: "double", Args: []Value{{Type: valueInt, Value: int64(5)}},
	}})
	enc.Encode(rpcRequest{JSONRPC: "2.0", ID: 2, Method: "function.call", Params: functionCallParams{
		Name: "double", Args: []Value{{Type: valueInt, Value: int64(10)}},
	}})
	enc.Encode(rpcRequest{JSONRPC: "2.0", ID: 3, Method: "plugin.shutdown", Params: nil})

	if err := server.RunIO(&input, &output); err != nil {
		t.Fatalf("RunIO: %v", err)
	}

	dec := json.NewDecoder(&output)
	var r1, r2 rpcResponse
	dec.Decode(&r1)
	dec.Decode(&r2)

	var v1, v2 Value
	json.Unmarshal(r1.Result, &v1)
	json.Unmarshal(r2.Result, &v2)

	if numberToInt64(v1.Value) != 10 {
		t.Errorf("first call: expected 10, got %v", v1)
	}
	if numberToInt64(v2.Value) != 20 {
		t.Errorf("second call: expected 20, got %v", v2)
	}
}

func TestServerEnvironmentMethods(t *testing.T) {
	resetGlobals(t)
	server := NewServer("env", "1.0.0", "test")

	for _, method := range []string{"environment.open", "environment.close"} {
		rpcErr := sendServerRequestExpectError(t, server, method, nil)
		if rpcErr != nil {
			t.Errorf("%s should succeed, got error: %v", method, rpcErr)
		}
	}
}

func TestServerShutdown(t *testing.T) {
	resetGlobals(t)
	server := NewServer("shut", "1.0.0", "test")

	var input bytes.Buffer
	var output bytes.Buffer
	enc := json.NewEncoder(&input)
	enc.Encode(rpcRequest{JSONRPC: "2.0", ID: 1, Method: "plugin.shutdown", Params: nil})

	if err := server.RunIO(&input, &output); err != nil {
		t.Fatalf("RunIO with shutdown: %v", err)
	}
}

func TestServerDoubleDestroy(t *testing.T) {
	resetGlobals(t)
	destroyed := 0
	class := object.NewClassBuilder("D").
		Method("__init__", func(self *object.Instance) {}).
		Method("__del__", func(self *object.Instance) { destroyed++ })

	server := NewServer("dbl", "1.0.0", "test").RegisterClass(class)

	ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{Class: "D"})

	sendServerRequest[any](t, server, "object.destroy", objectDestroyParams{ObjectID: ref.ID})
	sendServerRequest[any](t, server, "object.destroy", objectDestroyParams{ObjectID: ref.ID})

	if destroyed != 1 {
		t.Errorf("expected __del__ called once, got %d", destroyed)
	}
}

func TestServerClassWithKwargs(t *testing.T) {
	resetGlobals(t)
	class := object.NewClassBuilder("Opts").
		Method("__init__", func(self *object.Instance, kwargs object.Kwargs) {
			self.Fields["mode"] = object.NewString(kwargs.MustGetString("mode", "default"))
		}).
		Method("get_mode", func(self *object.Instance) string {
			return self.Fields["mode"].(*object.String).StringValue()
		})

	server := NewServer("kwargs", "1.0.0", "test").RegisterClass(class)

	ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{
		Class:  "Opts",
		Kwargs: map[string]Value{"mode": {Type: valueString, Value: "fast"}},
	})

	result := sendServerRequest[Value](t, server, "object.call_method", methodCallParams{
		ObjectID: ref.ID,
		Method:   "get_mode",
	})
	if result.Type != valueString || result.Value != "fast" {
		t.Fatalf("expected 'fast', got %#v", result)
	}
}

func TestServerMethodWithKwargs(t *testing.T) {
	resetGlobals(t)
	class := object.NewClassBuilder("KV").
		Method("__init__", func(self *object.Instance) {
			self.Fields["data"] = object.NewStringDict(map[string]object.Object{})
		}).
		Method("set_kwargs", func(self *object.Instance, kwargs object.Kwargs) {
			dict := self.Fields["data"].(*object.Dict)
			for k, v := range kwargs.Kwargs {
				dict.Pairs[k] = object.DictPair{Key: object.NewString(k), Value: v}
			}
		})

	server := NewServer("kwmethod", "1.0.0", "test").RegisterClass(class)

	ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{Class: "KV"})

	rpcErr := sendServerRequestExpectError(t, server, "object.call_method", methodCallParams{
		ObjectID: ref.ID,
		Method:   "set_kwargs",
		Kwargs:   map[string]Value{"name": {Type: valueString, Value: "Ada"}},
	})
	if rpcErr != nil {
		t.Fatalf("unexpected error: %v", rpcErr)
	}
}

func TestServerWrapperUnknownName(t *testing.T) {
	resetGlobals(t)
	fb := object.NewFunctionBuilder()
	fb.Function(func() string { return "hi" })

	server := NewServer("wn", "1.0.0", "test").
		RegisterFunc("greet", fb).
		Wrapper("nonexistent", `def bogus(): pass`)

	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{
		Protocol: ProtocolVersion, Transports: []string{"json"},
	})
	if len(result.Schema.Functions) != 1 || result.Schema.Functions[0].Mode != ModeRPC {
		t.Fatalf("wrapper on nonexistent name should be no-op")
	}
}

func TestServerEmptySchema(t *testing.T) {
	resetGlobals(t)
	server := NewServer("empty", "1.0.0", "test")
	result := sendServerRequest[handshakeResult](t, server, "scriptling.handshake", handshakeParams{
		Protocol: ProtocolVersion, Transports: []string{"json"},
	})
	if len(result.Schema.Functions) != 0 || len(result.Schema.Classes) != 0 || len(result.Schema.Constants) != 0 {
		t.Fatalf("expected empty schema")
	}
}
