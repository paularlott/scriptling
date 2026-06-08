package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

type testConfig struct {
	values map[string]any
}

func TestServerFunctionCall(t *testing.T) {
	server := NewServer("mathy", "1.0.0", "test math").
		Function("add", func(a int, b int) int {
			return a + b
		})

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
	destroyed := false
	server := NewServer("config", "1.0.0", "test config")
	server.Class("Config").
		Constructor(func(values map[string]any) *testConfig {
			return &testConfig{values: values}
		}).
		Method("get", func(c *testConfig, key string) any {
			return c.values[key]
		}).
		Destructor(func(c *testConfig) {
			destroyed = true
		})

	ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{
		EnvironmentID: "env-1",
		Class:         "Config",
		Args: []Value{{
			Type: valueDict,
			Entries: map[string]Value{
				"name": {Type: valueString, Value: "scriptling"},
			},
		}},
	})

	if ref.ID == "" || ref.Class != "Config" || ref.Library != "config" {
		t.Fatalf("unexpected ref: %#v", ref)
	}

	got := sendServerRequest[Value](t, server, "object.call_method", methodCallParams{
		EnvironmentID: "env-1",
		ObjectID:      ref.ID,
		Method:        "get",
		Args:          []Value{{Type: valueString, Value: "name"}},
	})
	if got.Type != valueString || got.Value != "scriptling" {
		t.Fatalf("expected string result, got %#v", got)
	}

	_ = sendServerRequest[any](t, server, "object.destroy", objectDestroyParams{
		EnvironmentID: "env-1",
		ObjectID:      ref.ID,
	})
	if !destroyed {
		t.Fatal("expected destructor to run")
	}
}

func TestServerHandshakeSchema(t *testing.T) {
	server := NewServer("hello", "1.2.3", "hello plugin").
		Function("greet", func(name string) string { return "hello " + name }).
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

func TestServerFunctionFromBuilder(t *testing.T) {
	builder := object.NewFunctionBuilder()
	builder.Function(func(name string) string {
		return "built " + name
	})

	server := NewServer("builtins", "1.0.0", "builder functions").
		RegisterFunc("label", builder.Build())

	result := sendServerRequest[Value](t, server, "function.call", functionCallParams{
		Name: "label",
		Args: []Value{{Type: valueString, Value: "Ada"}},
	})

	if result.Type != valueString || result.Value != "built Ada" {
		t.Fatalf("expected builder function result, got %#v", result)
	}
}

func TestServerClassFromBuilder(t *testing.T) {
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
		}).
		Build()

	server := NewServer("builderclass", "1.0.0", "builder class").
		RegisterClass(class)

	ref := sendServerRequest[RemoteRef](t, server, "object.new", objectNewParams{
		EnvironmentID: "env-1",
		Class:         "Counter",
		Args:          []Value{{Type: valueInt, Value: int64(4)}},
	})

	got := sendServerRequest[Value](t, server, "object.call_method", methodCallParams{
		EnvironmentID: "env-1",
		ObjectID:      ref.ID,
		Method:        "inc",
		Args:          []Value{{Type: valueInt, Value: int64(3)}},
	})
	if got.Type != valueInt || numberToInt64(got.Value) != 7 {
		t.Fatalf("expected counter value 7, got %#v", got)
	}

	got = sendServerRequest[Value](t, server, "object.call_method", methodCallParams{
		EnvironmentID: "env-1",
		ObjectID:      ref.ID,
		Method:        "get",
	})
	if got.Type != valueInt || numberToInt64(got.Value) != 7 {
		t.Fatalf("expected counter get 7, got %#v", got)
	}
}

func TestServerFunctionBuiltinNativeObjectAPI(t *testing.T) {
	server := NewServer("native", "1.0.0", "native object API").
		FunctionBuiltin("upper_label", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			name, err := args[0].AsString()
			if err != nil {
				return err
			}
			return object.NewString("native:" + name)
		})

	result := sendServerRequest[Value](t, server, "function.call", functionCallParams{
		Name: "upper_label",
		Args: []Value{{Type: valueString, Value: "Ada"}},
	})

	if result.Type != valueString || result.Value != "native:Ada" {
		t.Fatalf("expected native function result, got %#v", result)
	}
}

func TestServerEmbeddedScriptlingFunction(t *testing.T) {
	p := scriptling.New()
	if err := p.RegisterScriptFunc("decorate", `
def decorate(name):
    return "[" + name + "]"
`); err != nil {
		t.Fatalf("RegisterScriptFunc: %v", err)
	}

	server := NewServer("embedded", "1.0.0", "embedded scriptling").
		FunctionBuiltin("decorate", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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

	result := sendServerRequest[Value](t, server, "function.call", functionCallParams{
		Name: "decorate",
		Args: []Value{{Type: valueString, Value: "Ada"}},
	})

	if result.Type != valueString || result.Value != "[Ada]" {
		t.Fatalf("expected embedded Scriptling result, got %#v", result)
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
