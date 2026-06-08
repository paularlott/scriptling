package plugin

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func TestManagerLoadsExecutableAndRegistersProxyLibraries(t *testing.T) {
	if os.Getenv("SCRIPTLING_PLUGIN_TEST_HELPER") == "1" {
		runPluginTestHelper()
		return
	}
	if os.Getenv("SCRIPTLING_PLUGIN_CALLBACK_HELPER") == "1" {
		runCallbackPluginTestHelper()
		return
	}
	if os.Getenv("SCRIPTLING_PLUGIN_WRAPPER_HELPER") == "1" {
		runWrapperPluginTestHelper()
		return
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "hello-plugin")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writePluginHelper(t, helper)

	manager := NewManager()
	manager.AddDir(dir)
	if err := manager.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	defer manager.Close()

	if warnings := manager.Warnings(); len(warnings) != 0 {
		t.Fatalf("unexpected warnings: %#v", warnings)
	}
	if plugins := manager.List(); len(plugins) != 1 || plugins[0].Name != "plugin.hello" {
		t.Fatalf("unexpected plugin list: %#v", plugins)
	}

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import plugin.hello

cfg = plugin.hello.Config({"name": "Ada"})
plugin.hello.greet(cfg.get("name"))
`)
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok || str.StringValue() != "Hello, Ada" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestPluginSuppliedWrapperSource(t *testing.T) {
	if os.Getenv("SCRIPTLING_PLUGIN_WRAPPER_HELPER") == "1" {
		runWrapperPluginTestHelper()
		return
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "wrapper-plugin")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeWrapperPluginHelper(t, helper)

	manager := NewManager()
	manager.AddDir(dir)
	if err := manager.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import plugin.wrap
plugin.wrap.greet("Ada")
`)
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok || str.StringValue() != "Hello, Ada!" {
		t.Fatalf("unexpected wrapper result: %#v", result)
	}
}

func TestCallbackArgumentDuringRunningPluginCall(t *testing.T) {
	if os.Getenv("SCRIPTLING_PLUGIN_CALLBACK_HELPER") == "1" {
		runCallbackPluginTestHelper()
		return
	}

	dir := t.TempDir()
	helper := filepath.Join(dir, "stream-plugin")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeCallbackPluginHelper(t, helper)

	manager := NewManager()
	manager.AddDir(dir)
	if err := manager.Load(context.Background()); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import plugin.stream

chunks = []
def on_chunk(text):
    append(chunks, text)

plugin.stream.complete("ignored", on_chunk)
"".join(chunks)
`)
	if err != nil {
		t.Fatalf("Eval returned error: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok || str.StringValue() != "hello stream" {
		t.Fatalf("unexpected callback result: %#v", result)
	}
}

func writePluginHelper(t *testing.T, path string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_PLUGIN_TEST_HELPER=1\r\n\"" + exe + "\" -test.run=TestManagerLoadsExecutableAndRegistersProxyLibraries --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_PLUGIN_TEST_HELPER=1 exec \"" + exe + "\" -test.run=TestManagerLoadsExecutableAndRegistersProxyLibraries --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

func writeCallbackPluginHelper(t *testing.T, path string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_PLUGIN_CALLBACK_HELPER=1\r\n\"" + exe + "\" -test.run=TestCallbackArgumentDuringRunningPluginCall --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_PLUGIN_CALLBACK_HELPER=1 exec \"" + exe + "\" -test.run=TestCallbackArgumentDuringRunningPluginCall --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write callback helper: %v", err)
	}
}

func writeWrapperPluginHelper(t *testing.T, path string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_PLUGIN_WRAPPER_HELPER=1\r\n\"" + exe + "\" -test.run=TestPluginSuppliedWrapperSource --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_PLUGIN_WRAPPER_HELPER=1 exec \"" + exe + "\" -test.run=TestPluginSuppliedWrapperSource --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write wrapper helper: %v", err)
	}
}

func runPluginTestHelper() {
	type config struct {
		values map[string]any
	}
	server := NewServer("hello", "1.0.0", "test helper plugin")
	server.Function("greet", func(name string) string {
		return "Hello, " + name
	})
	server.Class("Config").
		Constructor(func(values map[string]any) *config {
			return &config{values: values}
		}).
		Method("get", func(c *config, key string) any {
			return c.values[key]
		})
	_ = server.Run()
	os.Exit(0)
}

func runCallbackPluginTestHelper() {
	decoder := json.NewDecoder(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)
	for {
		var req rpcRequest
		if err := decoder.Decode(&req); err != nil {
			if err == io.EOF {
				return
			}
			os.Exit(1)
		}
		switch req.Method {
		case "scriptling.handshake":
			_ = encoder.Encode(rpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: mustRawJSON(handshakeResult{
					Protocol:  ProtocolVersion,
					Transport: "json",
					Library: libraryInfo{
						Name:        "plugin.stream",
						Version:     "1.0.0",
						Description: "stream callback test plugin",
					},
					Capabilities: []string{"callbacks"},
					Schema: Schema{
						Functions: []FunctionSchema{{Name: "complete", Wrapper: "generated"}},
					},
				}),
			})
		case "function.call":
			var params functionCallParams
			raw, _ := json.Marshal(req.Params)
			_ = json.Unmarshal(raw, &params)
			callbackID, _ := params.Args[1].Value.(string)
			for i, chunk := range []string{"hello ", "stream"} {
				_ = encoder.Encode(rpcRequest{
					JSONRPC: "2.0",
					ID:      int64(500 + i),
					Method:  "callback.call",
					Params: callbackCallParams{
						EnvironmentID: params.EnvironmentID,
						CallbackID:    callbackID,
						Args:          []Value{{Type: valueString, Value: chunk}},
					},
				})
				var callbackResp rpcResponse
				_ = decoder.Decode(&callbackResp)
			}
			_ = encoder.Encode(rpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result:  mustRawJSON(Value{Type: valueString, Value: "done"}),
			})
		case "plugin.shutdown":
			_ = encoder.Encode(rpcResponse{JSONRPC: "2.0", ID: req.ID})
			return
		default:
			_ = encoder.Encode(rpcResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &RPCError{Code: -32601, Message: "unknown method"},
			})
		}
	}
}

func runWrapperPluginTestHelper() {
	server := NewServer("wrap", "1.0.0", "wrapper test plugin")
	server.HiddenFunction("_greet", func(name string) string {
		return "Hello, " + name
	})
	server.Wrapper("greet", `
import scriptling.plugin

def greet(name):
    return scriptling.plugin.call_function("plugin.wrap", "_greet", name) + "!"
`)
	_ = server.Run()
	os.Exit(0)
}

func mustRawJSON(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}
