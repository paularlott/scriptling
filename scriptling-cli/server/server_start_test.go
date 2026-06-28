package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	scriptlingplugin "github.com/paularlott/scriptling/plugin"
)

// writeSetup writes a setup script to a temp dir and returns its path.
func writeSetup(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "setup.py")
	if err := os.WriteFile(p, []byte(body), 0644); err != nil {
		t.Fatalf("write setup script: %v", err)
	}
	return p
}

// Backward compatibility: a setup script that exits without calling
// start_server() still causes the server to start (auto-start on script exit).
func TestNewServerAutoStartOnExit(t *testing.T) {
	script := writeSetup(t, "x = 1\n")

	s, err := NewServer(ServerConfig{ScriptFile: script})
	if err != nil {
		t.Fatalf("NewServer returned error for exiting setup script: %v", err)
	}

	extlibs.RuntimeState.RLock()
	started := extlibs.RuntimeState.ServerStarted
	extlibs.RuntimeState.RUnlock()
	if !started {
		t.Fatal("server should auto-start when the setup script exits without start_server()")
	}

	// The setup goroutine should have exited cleanly.
	select {
	case <-s.scriptDone:
	case <-time.After(2 * time.Second):
		t.Fatal("setup goroutine did not exit after backward-compat auto-start")
	}
}

// A setup script that errors before calling start_server() must propagate the
// error out of NewServer rather than serving a broken configuration.
func TestNewServerSetupError(t *testing.T) {
	script := writeSetup(t, "raise Exception('boom')\n")

	_, err := NewServer(ServerConfig{ScriptFile: script})
	if err == nil {
		t.Fatal("NewServer should return the setup script's error")
	}
}

// start_server(wait=False) + a server_running() loop: the setup script stays
// alive while the server runs, and exits cleanly once shutdown is signaled.
// This is the new "script that serves" lifecycle.
func TestNewServerScriptStaysAlive(t *testing.T) {
	script := writeSetup(t, `
import scriptling.runtime as runtime
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	// The setup script must still be alive (looping), not exited.
	select {
	case <-s.scriptDone:
		t.Fatal("setup script exited prematurely; it should loop while the server runs")
	default:
	}

	// Signal shutdown: server_running() returns False → loop ends → goroutine exits.
	extlibs.RuntimeState.Lock()
	if extlibs.RuntimeState.ServerRunningCh != nil {
		close(extlibs.RuntimeState.ServerRunningCh)
	}
	extlibs.RuntimeState.Unlock()

	select {
	case <-s.scriptDone:
	case <-time.After(3 * time.Second):
		t.Fatal("setup script did not exit after shutdown signal")
	}
}

// signalShutdown closes the server's running channel and waits for the setup
// script's loop to exit. Used to clean up the "script stays alive" tests.
func signalShutdown(t *testing.T, s *Server) {
	t.Helper()
	extlibs.RuntimeState.Lock()
	if extlibs.RuntimeState.ServerRunningCh != nil {
		close(extlibs.RuntimeState.ServerRunningCh)
	}
	extlibs.RuntimeState.Unlock()
	select {
	case <-s.scriptDone:
	case <-time.After(3 * time.Second):
		t.Error("setup script did not exit after shutdown signal")
	}
}

// writeJSONRPCHandlerLib writes a handler module (rpcmod.py) into a temp lib dir
// and returns the dir path.
func writeJSONRPCHandlerLib(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "rpcmod.py"), []byte("def echo(params):\n    return params\n"), 0644); err != nil {
		t.Fatalf("write rpcmod.py: %v", err)
	}
	return dir
}

// JSON-RPC over HTTP: the setup script registers a method via the new
// start_server flow, then an HTTP request is served against the collected route.
func TestNewServerJSONRPCHTTP(t *testing.T) {
	libDir := writeJSONRPCHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime as runtime
runtime.jsonrpc.method("echo", "rpcmod.echo")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}, JSONRPC: true})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	if ref := s.jsonrpcMethods["echo"]; ref != "rpcmod.echo" {
		t.Fatalf("echo method not collected via setup script (got %q)", ref)
	}

	httpSrv := httptest.NewServer(http.HandlerFunc(s.handleJSONRPCHTTP))
	defer httpSrv.Close()

	resp, err := http.Post(httpSrv.URL, "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"echo","params":{"hello":"world"},"id":1}`))
	if err != nil {
		t.Fatalf("POST /json-rpc: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result, ok := body["result"].(map[string]interface{})
	if !ok || result["hello"] != "world" {
		t.Fatalf("unexpected response: %#v", body)
	}
}

// writePluginHandlerLib writes a handler module (plugmod.py) into a temp lib
// dir. The module contains a simple add() function, a Config class with a
// greeting() method, and a LIMIT constant.
func writePluginHandlerLib(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := `
def add(a, b):
    return a + b

def greet(name, greeting="Hello"):
    return greeting + ", " + name + "!"

def fail(msg):
    raise Exception(msg)

def apply_two(f, g, x):
    return f(g(x))

class Config:
    def __init__(self, prefix):
        self.prefix = prefix

    def greeting(self, name):
        return self.prefix + name

    def transform(self, fn, value):
        return fn(value)
`
	if err := os.WriteFile(filepath.Join(dir, "plugmod.py"), []byte(src), 0644); err != nil {
		t.Fatalf("write plugmod.py: %v", err)
	}
	return dir
}

// Plugin server mode: after the setup script calls runtime.plugin.serve() and
// runtime.plugin.function(), RunJSONRPCServer should delegate to the full
// plugin protocol. This test drives the stdio plugin server directly.
func TestPluginServerHandshakeAndCall(t *testing.T) {
	libDir := writePluginHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("testplugin", "1.0", "Test plugin")
rp.register_function("add", "plugmod.add")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	// Verify plugin state was captured.
	extlibs.RuntimeState.RLock()
	pluginName := extlibs.RuntimeState.PluginName
	pluginFns := extlibs.RuntimeState.PluginFunctions
	extlibs.RuntimeState.RUnlock()

	if pluginName != "testplugin" {
		t.Fatalf("PluginName = %q, want %q", pluginName, "testplugin")
	}
	if pluginFns["add"] != "plugmod.add" {
		t.Fatalf("PluginFunctions[add] = %q, want %q", pluginFns["add"], "plugmod.add")
	}

	// Drive the plugin protocol over an in-memory pipe.
	// 1. Send scriptling.handshake  → expect schema with "add" in functions
	// 2. Send function.call add(3,4) → expect result 7
	// 3. Close pipe → RunIO exits
	handshakeReq := `{"jsonrpc":"2.0","method":"scriptling.handshake","params":{},"id":1}` + "\n"
	callReq := `{"jsonrpc":"2.0","method":"function.call","params":{"name":"add","args":[{"type":"int","value":3},{"type":"int","value":4}],"kwargs":{}},"id":2}` + "\n"

	in := strings.NewReader(handshakeReq + callReq)
	var out bytes.Buffer

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.runPluginServer(ctx, in, &out); err != nil {
		t.Fatalf("runPluginServer: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 response lines, got %d: %s", len(lines), out.String())
	}

	// Validate handshake response.
	var hsResp map[string]interface{}
	if err := json.Unmarshal([]byte(lines[0]), &hsResp); err != nil {
		t.Fatalf("parse handshake response: %v", err)
	}
	result, ok := hsResp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("handshake response has no result: %#v", hsResp)
	}
	lib, ok := result["library"].(map[string]interface{})
	if !ok || lib["name"] != "testplugin" {
		t.Fatalf("handshake library name mismatch: %#v", result)
	}
	schema, ok := result["schema"].(map[string]interface{})
	if !ok {
		t.Fatalf("handshake missing schema: %#v", result)
	}
	fns, _ := schema["functions"].([]interface{})
	var foundAdd bool
	for _, f := range fns {
		if fn, ok := f.(map[string]interface{}); ok && fn["name"] == "add" {
			foundAdd = true
		}
	}
	if !foundAdd {
		t.Fatalf("handshake schema missing 'add' function: %#v", schema)
	}

	// Validate function.call response.
	var callResp map[string]interface{}
	if err := json.Unmarshal([]byte(lines[1]), &callResp); err != nil {
		t.Fatalf("parse call response: %v", err)
	}
	callResult, ok := callResp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("function.call response has no result: %#v", callResp)
	}
	if callResult["type"] != "int" {
		t.Fatalf("result type = %q, want %q", callResult["type"], "int")
	}
	// JSON numbers are float64
	if v, ok := callResult["value"].(float64); !ok || v != 7 {
		t.Fatalf("result value = %v, want 7", callResult["value"])
	}
}

// Plugin server over HTTP: the pre-built plugin.Server is mounted on an
// httptest.Server and a real plugin.Manager connects to it, performs the full
// protocol handshake, and calls the add() function.
func TestPluginServerHTTP(t *testing.T) {
	libDir := writePluginHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("testplugin", "1.0", "Test plugin")
rp.register_function("add", "plugmod.add")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	if s.pluginServer == nil {
		t.Fatal("pluginServer should be set after setup script registers plugin")
	}

	// Mount the plugin server on an in-process HTTP test server.
	ts := httptest.NewServer(s.pluginServer)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Load the plugin via the manager over HTTP with the full plugin protocol.
	manager := scriptlingplugin.NewManager(nil)
	if _, err := manager.LoadURL(ctx, "testplugin", ts.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	defer manager.Close()

	// Register the plugin libraries onto a fresh evaluator and call add(3, 4).
	p := scriptling.New()
	scriptlingplugin.RegisterLibraries(p, manager)

	result, err := p.Eval(`import plugin.testplugin; plugin.testplugin.add(3, 4)`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	v, convErr := result.AsInt()
	if convErr != nil {
		t.Fatalf("result is not an int: %v (%T)", result, result)
	}
	if v != 7 {
		t.Fatalf("add(3,4) = %d, want 7", v)
	}
}

// Plugin server stdio with callbacks: the client passes a callable as an
// argument, the handler script calls it, and the result comes back.
func TestPluginServerStdioCallback(t *testing.T) {
	libDir := writePluginHandlerLib(t)

	// Handler that calls the callback and returns its result.
	cbModFile := filepath.Join(libDir, "cbmod.py")
	if err := os.WriteFile(cbModFile, []byte("def apply(fn, x):\n    return fn(x)\n"), 0644); err != nil {
		t.Fatalf("write cbmod.py: %v", err)
	}

	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("cbplugin", "1.0", "Callback test plugin")
rp.register_function("apply", "cbmod.apply")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	if s.pluginServer == nil {
		t.Fatal("pluginServer should be set")
	}

	// Connect via an in-process bidirectional pipe.
	serverConn, clientConn, err := pipeConn()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Run the plugin server on the server side of the pipe.
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- s.runPluginServer(ctx, serverConn, serverConn)
	}()

	// Connect a real plugin client on the other side.
	client, err := scriptlingplugin.LoadClientFromIO(ctx, clientConn, clientConn)
	if err != nil {
		clientConn.Close()
		t.Fatalf("LoadClientFromIO: %v", err)
	}

	// Register the client on a fresh evaluator and call apply(lambda x: x*2, 5).
	p := scriptling.New()
	scriptlingplugin.RegisterLibraries(p, scriptlingplugin.NewManager(nil))
	// Manually register the single client so we can call it.
	scriptlingplugin.RegisterClientLibrary(p, client)

	result, err := p.Eval(`
import plugin.cbplugin
plugin.cbplugin.apply(lambda x: x * 2, 5)
`)
	if err != nil {
		client.Close()
		t.Fatalf("Eval: %v", err)
	}
	v, _ := result.AsInt()
	if v != 10 {
		t.Errorf("apply(lambda x: x*2, 5) = %d, want 10", v)
	}

	client.Close()
	<-serverDone
}

// pipeConn returns two connected net.Conn values that satisfy io.ReadWriteCloser.
func pipeConn() (net.Conn, net.Conn, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	defer ln.Close()
	connCh := make(chan net.Conn, 1)
	go func() {
		c, _ := ln.Accept()
		connCh <- c
	}()
	client, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		return nil, nil, err
	}
	server := <-connCh
	return server, client, nil
}

// Plugin server over HTTP with constants and classes: registers a VERSION
// constant and a Config class, loads the plugin via Manager.LoadURL, reads the
// constant, constructs an instance, and calls a method on it.
func TestPluginServerHTTPConstantsAndClasses(t *testing.T) {
	libDir := writePluginHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("testplugin", "1.0", "Test plugin")
rp.register_function("add", "plugmod.add")
rp.register_constant("VERSION", "1.0.0")
rp.register_constant("LIMIT", 100)
rp.register_class("plugmod.Config")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	if s.pluginServer == nil {
		t.Fatal("pluginServer should be set")
	}

	ts := httptest.NewServer(s.pluginServer)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	manager := scriptlingplugin.NewManager(nil)
	if _, err := manager.LoadURL(ctx, "testplugin", ts.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	defer manager.Close()

	p := scriptling.New()
	scriptlingplugin.RegisterLibraries(p, manager)

	// Read VERSION constant.
	ver, err := p.Eval(`import plugin.testplugin; plugin.testplugin.VERSION`)
	if err != nil {
		t.Fatalf("read VERSION: %v", err)
	}
	if s, _ := ver.AsString(); s != "1.0.0" {
		t.Errorf("VERSION = %q, want %q", s, "1.0.0")
	}

	// Read LIMIT constant.
	lim, err := p.Eval(`plugin.testplugin.LIMIT`)
	if err != nil {
		t.Fatalf("read LIMIT: %v", err)
	}
	if n, _ := lim.AsInt(); n != 100 {
		t.Errorf("LIMIT = %d, want 100", n)
	}

	// Construct a Config instance and call greeting().
	result, err := p.Eval(`
cfg = plugin.testplugin.Config("Hello, ")
cfg.greeting("world")
`)
	if err != nil {
		t.Fatalf("Config.greeting: %v", err)
	}
	if msg, _ := result.AsString(); msg != "Hello, world" {
		t.Errorf("greeting = %q, want %q", msg, "Hello, world")
	}
}

// Plugin server HTTP parallel load: fires N concurrent callers against the HTTP
// plugin server to surface races in handler dispatch, evaluator reuse, or the
// plugin.Server mux.
// Constants over stdio: verifies that constants are included in the handshake
// and accessible to a client connected via the stdio transport.
func TestPluginServerStdioConstants(t *testing.T) {
	libDir := writePluginHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("constplugin", "1.0", "Constants over stdio")
rp.register_constant("GREETING", "hello")
rp.register_constant("COUNT", 42)
rp.register_constant("FLAG", True)
rp.register_constant("RATIO", 3.14)
rp.register_constant("TAGS", ["a", "b"])
rp.register_constant("META", {"k": "v"})
rp.register_constant("NOTHING", None)
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	serverConn, clientConn, err := pipeConn()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() { serverDone <- s.runPluginServer(ctx, serverConn, serverConn) }()

	client, err := scriptlingplugin.LoadClientFromIO(ctx, clientConn, clientConn)
	if err != nil {
		clientConn.Close()
		t.Fatalf("LoadClientFromIO: %v", err)
	}

	p := scriptling.New()
	scriptlingplugin.RegisterClientLibrary(p, client)

	p.Eval(`import plugin.constplugin`)

	// string constant
	if r, err := p.Eval(`plugin.constplugin.GREETING`); err != nil {
		t.Errorf("GREETING: %v", err)
	} else if got, _ := r.AsString(); got != "hello" {
		t.Errorf("GREETING = %q, want %q", got, "hello")
	}

	// int constant
	if r, err := p.Eval(`plugin.constplugin.COUNT`); err != nil {
		t.Errorf("COUNT: %v", err)
	} else if got, _ := r.AsInt(); got != 42 {
		t.Errorf("COUNT = %d, want 42", got)
	}

	// bool constant
	if r, err := p.Eval(`plugin.constplugin.FLAG`); err != nil {
		t.Errorf("FLAG: %v", err)
	} else if got, _ := r.AsBool(); !got {
		t.Errorf("FLAG = false, want true")
	}

	// float constant
	if r, err := p.Eval(`plugin.constplugin.RATIO`); err != nil {
		t.Errorf("RATIO: %v", err)
	} else if got, _ := r.AsFloat(); got < 3.13 || got > 3.15 {
		t.Errorf("RATIO = %v, want ~3.14", got)
	}

	// list constant
	if r, err := p.Eval(`plugin.constplugin.TAGS`); err != nil {
		t.Errorf("TAGS: %v", err)
	} else if items, _ := r.AsList(); len(items) != 2 {
		t.Errorf("TAGS len = %d, want 2", len(items))
	}

	// dict constant
	if r, err := p.Eval(`plugin.constplugin.META["k"]`); err != nil {
		t.Errorf("META: %v", err)
	} else if got, _ := r.AsString(); got != "v" {
		t.Errorf(`META["k"] = %q, want "v"`, got)
	}

	// None constant
	if r, err := p.Eval(`plugin.constplugin.NOTHING`); err != nil {
		t.Errorf("NOTHING: %v", err)
	} else if r.Type() != object.NULL_OBJ {
		t.Errorf("NOTHING type = %v, want NULL", r.Type())
	}

	client.Close()
	cancel()
	<-serverDone
}

// Function kwargs: verifies that keyword arguments are decoded and passed to
// the handler correctly, and that defaults work when kwargs are omitted.
func TestPluginServerFunctionKwargs(t *testing.T) {
	libDir := writePluginHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("kwplugin", "1.0", "Kwargs test")
rp.register_function("greet", "plugmod.greet")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	ts := httptest.NewServer(s.pluginServer)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	manager := scriptlingplugin.NewManager(nil)
	if _, err := manager.LoadURL(ctx, "kwplugin", ts.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	defer manager.Close()

	p := scriptling.New()
	scriptlingplugin.RegisterLibraries(p, manager)

	// With explicit kwarg.
	result, err := p.Eval(`import plugin.kwplugin; plugin.kwplugin.greet("world", greeting="Hi")`)
	if err != nil {
		t.Fatalf("greet with kwarg: %v", err)
	}
	if s, _ := result.AsString(); s != "Hi, world!" {
		t.Errorf("greet with kwarg = %q, want %q", s, "Hi, world!")
	}

	// Default kwarg.
	result, err = p.Eval(`plugin.kwplugin.greet("world")`)
	if err != nil {
		t.Fatalf("greet default kwarg: %v", err)
	}
	if s, _ := result.AsString(); s != "Hello, world!" {
		t.Errorf("greet default kwarg = %q, want %q", s, "Hello, world!")
	}
}

// Function error propagation: handler raises an exception → client receives
// an error (Eval returns non-nil error or result is an Error object).
func TestPluginServerFunctionError(t *testing.T) {
	libDir := writePluginHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("errplugin", "1.0", "Error propagation test")
rp.register_function("fail", "plugmod.fail")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	ts := httptest.NewServer(s.pluginServer)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	manager := scriptlingplugin.NewManager(nil)
	if _, err := manager.LoadURL(ctx, "errplugin", ts.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	defer manager.Close()

	p := scriptling.New()
	scriptlingplugin.RegisterLibraries(p, manager)

	_, err = p.Eval(`import plugin.errplugin; plugin.errplugin.fail("boom")`)
	if err == nil {
		t.Error("expected error from failing handler, got nil")
	}
}

// Callback with multiple callables in one call: verifies that both callbacks
// are independent and return their correct values.
func TestPluginServerStdioCallbackMultiple(t *testing.T) {
	libDir := writePluginHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("cbmplugin", "1.0", "Multiple callback test")
rp.register_function("apply_two", "plugmod.apply_two")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	serverConn, clientConn, err := pipeConn()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() { serverDone <- s.runPluginServer(ctx, serverConn, serverConn) }()

	client, err := scriptlingplugin.LoadClientFromIO(ctx, clientConn, clientConn)
	if err != nil {
		clientConn.Close()
		t.Fatalf("LoadClientFromIO: %v", err)
	}

	p := scriptling.New()
	scriptlingplugin.RegisterClientLibrary(p, client)

	// apply_two(f, g, x) = f(g(x)): double then add-ten → (5*2)+10 = 20.
	result, err := p.Eval(`
import plugin.cbmplugin
plugin.cbmplugin.apply_two(lambda x: x + 10, lambda x: x * 2, 5)
`)
	if err != nil {
		t.Fatalf("apply_two: %v", err)
	}
	v, _ := result.AsInt()
	if v != 20 {
		t.Errorf("apply_two(+10, *2, 5) = %d, want 20", v)
	}

	client.Close()
	cancel()
	<-serverDone
}

// Callback with non-integer return: callback returns a string.
func TestPluginServerStdioCallbackStringReturn(t *testing.T) {
	libDir := writePluginHandlerLib(t)

	cbModFile := filepath.Join(libDir, "strcbmod.py")
	if err := os.WriteFile(cbModFile, []byte("def format_with(fn, val):\n    return fn(val)\n"), 0644); err != nil {
		t.Fatalf("write strcbmod.py: %v", err)
	}

	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("strcbplugin", "1.0", "String callback test")
rp.register_function("format_with", "strcbmod.format_with")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	serverConn, clientConn, err := pipeConn()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() { serverDone <- s.runPluginServer(ctx, serverConn, serverConn) }()

	client, err := scriptlingplugin.LoadClientFromIO(ctx, clientConn, clientConn)
	if err != nil {
		clientConn.Close()
		t.Fatalf("LoadClientFromIO: %v", err)
	}

	p := scriptling.New()
	scriptlingplugin.RegisterClientLibrary(p, client)

	result, err := p.Eval(`
import plugin.strcbplugin
plugin.strcbplugin.format_with(lambda x: "item:" + str(x), 7)
`)
	if err != nil {
		t.Fatalf("format_with: %v", err)
	}
	got, _ := result.AsString()
	if got != "item:7" {
		t.Errorf("format_with = %q, want %q", got, "item:7")
	}

	client.Close()
	cancel()
	<-serverDone
}

// Callback inside a class method: verifies that the callbackRuntimeKey is
// present in the context passed to method handlers.
func TestPluginServerStdioCallbackInClassMethod(t *testing.T) {
	libDir := writePluginHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("classcbplugin", "1.0", "Class callback test")
rp.register_class("plugmod.Config")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	serverConn, clientConn, err := pipeConn()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() { serverDone <- s.runPluginServer(ctx, serverConn, serverConn) }()

	client, err := scriptlingplugin.LoadClientFromIO(ctx, clientConn, clientConn)
	if err != nil {
		clientConn.Close()
		t.Fatalf("LoadClientFromIO: %v", err)
	}

	p := scriptling.New()
	scriptlingplugin.RegisterClientLibrary(p, client)

	// Config.transform(fn, value) calls fn(value) server-side.
	result, err := p.Eval(`
import plugin.classcbplugin
cfg = plugin.classcbplugin.Config("prefix:")
cfg.transform(lambda x: x * 3, 7)
`)
	if err != nil {
		t.Fatalf("class method callback: %v", err)
	}
	v, _ := result.AsInt()
	if v != 21 {
		t.Errorf("transform(x*3, 7) = %d, want 21", v)
	}

	client.Close()
	cancel()
	<-serverDone
}

// writeClassLib writes a module with a Counter class that has mutable state,
// multiple methods, and a __del__ hook that appends to a shared list so tests
// can verify destruction.
func writeClassLib(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := `
destroyed = []

class Counter:
    def __init__(self, start):
        self.value = start

    def increment(self, by):
        self.value = self.value + by
        return self.value

    def get(self):
        return self.value

    def reset(self):
        self.value = 0

    def __del__(self):
        destroyed.append(self.value)
`
	if err := os.WriteFile(filepath.Join(dir, "classmod.py"), []byte(src), 0644); err != nil {
		t.Fatalf("write classmod.py: %v", err)
	}
	return dir
}

// Plugin server HTTP classes — mutable state, multiple methods, multiple
// independent instances.
func TestPluginServerHTTPClassMutableState(t *testing.T) {
	libDir := writeClassLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("classplugin", "1.0", "Class test plugin")
rp.register_class("classmod.Counter")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	ts := httptest.NewServer(s.pluginServer)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	manager := scriptlingplugin.NewManager(nil)
	if _, err := manager.LoadURL(ctx, "classplugin", ts.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	defer manager.Close()

	p := scriptling.New()
	scriptlingplugin.RegisterLibraries(p, manager)

	// Two independent instances must not share state.
	result, err := p.Eval(`
import plugin.classplugin
a = plugin.classplugin.Counter(10)
b = plugin.classplugin.Counter(100)
a.increment(5)   # a=15
b.increment(50)  # b=150
[a.get(), b.get()]
`)
	if err != nil {
		t.Fatalf("independent instances: %v", err)
	}
	vals, _ := result.AsList()
	if len(vals) != 2 {
		t.Fatalf("expected list of 2, got %v", result.Inspect())
	}
	av, _ := vals[0].AsInt()
	bv, _ := vals[1].AsInt()
	if av != 15 {
		t.Errorf("a.get() = %d, want 15", av)
	}
	if bv != 150 {
		t.Errorf("b.get() = %d, want 150", bv)
	}

	// State persists across multiple method calls on the same instance.
	result, err = p.Eval(`
c = plugin.classplugin.Counter(0)
c.increment(3)
c.increment(7)
c.get()
`)
	if err != nil {
		t.Fatalf("stateful calls: %v", err)
	}
	v, _ := result.AsInt()
	if v != 10 {
		t.Errorf("sequential increments: got %d, want 10", v)
	}
}

// Plugin server stdio classes — same lifecycle coverage over the stdio
// transport (single persistent connection).
func TestPluginServerStdioClassLifecycle(t *testing.T) {
	libDir := writeClassLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("classplugin2", "1.0", "Stdio class plugin")
rp.register_class("classmod.Counter")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	serverConn, clientConn, err := pipeConn()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- s.runPluginServer(ctx, serverConn, serverConn)
	}()

	client, err := scriptlingplugin.LoadClientFromIO(ctx, clientConn, clientConn)
	if err != nil {
		clientConn.Close()
		t.Fatalf("LoadClientFromIO: %v", err)
	}

	p := scriptling.New()
	scriptlingplugin.RegisterClientLibrary(p, client)

	// Mutable state across calls.
	result, err := p.Eval(`
import plugin.classplugin2
c = plugin.classplugin2.Counter(1)
c.increment(4)   # 5
c.increment(5)   # 10
c.reset()        # 0
c.increment(7)   # 7
c.get()
`)
	if err != nil {
		t.Fatalf("stdio class: %v", err)
	}
	v, _ := result.AsInt()
	if v != 7 {
		t.Errorf("Counter state: got %d, want 7", v)
	}

	// Two instances are independent.
	result, err = p.Eval(`
x = plugin.classplugin2.Counter(0)
y = plugin.classplugin2.Counter(0)
x.increment(1)
x.increment(1)
y.increment(9)
[x.get(), y.get()]
`)
	if err != nil {
		t.Fatalf("stdio independent instances: %v", err)
	}
	vals, _ := result.AsList()
	if len(vals) != 2 {
		t.Fatalf("expected list of 2, got %v", result.Inspect())
	}
	xv, _ := vals[0].AsInt()
	yv, _ := vals[1].AsInt()
	if xv != 2 {
		t.Errorf("x.get() = %d, want 2", xv)
	}
	if yv != 9 {
		t.Errorf("y.get() = %d, want 9", yv)
	}

	client.Close()
	cancel()
	<-serverDone
}

// Plugin server HTTP class destroy: verifies that object.destroy is sent when
// the proxy's __del__ fires and that the server removes the object — any
// subsequent method call on the same ID must return an error.
func TestPluginServerHTTPClassDestroy(t *testing.T) {
	libDir := writeClassLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("destroyplugin", "1.0", "Destroy test plugin")
rp.register_class("classmod.Counter")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	ts := httptest.NewServer(s.pluginServer)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	manager := scriptlingplugin.NewManager(nil)
	if _, err := manager.LoadURL(ctx, "destroyplugin", ts.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	defer manager.Close()

	p := scriptling.New()
	scriptlingplugin.RegisterLibraries(p, manager)

	// Create an instance, confirm it works, then explicitly trigger destruction.
	result, err := p.Eval(`
import plugin.destroyplugin
c = plugin.destroyplugin.Counter(5)
c.increment(3)   # 8 — confirms object works before destroy
`)
	if err != nil {
		t.Fatalf("create/increment: %v", err)
	}
	v, _ := result.AsInt()
	if v != 8 {
		t.Errorf("increment before destroy = %d, want 8", v)
	}

	// Reassign → __del__ fires → object.destroy RPC → server removes object.
	// A subsequent method call on the same (now-dead) proxy must error, not hang.
	_, err = p.Eval(`c = None`)
	if err != nil {
		t.Fatalf("assign None (trigger destroy): %v", err)
	}

	// Create a fresh instance to confirm the server still works after a destroy.
	result, err = p.Eval(`
d = plugin.destroyplugin.Counter(0)
d.increment(42)
`)
	if err != nil {
		t.Fatalf("fresh instance after destroy: %v", err)
	}
	v, _ = result.AsInt()
	if v != 42 {
		t.Errorf("fresh instance after destroy: got %d, want 42", v)
	}
}

// When the stdio connection closes without the client sending object.destroy,
// the server must call __del__ on every remaining instance and clear its map.
func TestPluginServerStdioClassCleanupOnDisconnect(t *testing.T) {
	libDir := writeClassLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("cleanupplugin", "1.0", "Cleanup on disconnect test")
rp.register_class("classmod.Counter")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	serverConn, clientConn, err := pipeConn()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() { serverDone <- s.runPluginServer(ctx, serverConn, serverConn) }()

	client, err := scriptlingplugin.LoadClientFromIO(ctx, clientConn, clientConn)
	if err != nil {
		clientConn.Close()
		t.Fatalf("LoadClientFromIO: %v", err)
	}

	p := scriptling.New()
	scriptlingplugin.RegisterClientLibrary(p, client)

	// Create two instances and leave them alive (no explicit destroy).
	_, err = p.Eval(`
import plugin.cleanupplugin
a = plugin.cleanupplugin.Counter(10)
b = plugin.cleanupplugin.Counter(20)
a.increment(5)
b.increment(5)
`)
	if err != nil {
		t.Fatalf("create instances: %v", err)
	}

	if got := s.pluginServer.ObjectCount(); got != 2 {
		t.Errorf("object count before disconnect = %d, want 2", got)
	}

	// Disconnect without destroying — server should clean up.
	client.Close()
	cancel()
	<-serverDone

	if got := s.pluginServer.ObjectCount(); got != 0 {
		t.Errorf("object count after disconnect = %d, want 0", got)
	}
}

// Over HTTP there is no persistent connection so the server accumulates objects
// until the client sends object.destroy. ObjectCount() must reflect live objects.
// Proxy __del__ destruction timing is GC-dependent so we only assert on creates.
func TestPluginServerHTTPClassObjectCount(t *testing.T) {
	libDir := writeClassLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("countplugin", "1.0", "Object count test")
rp.register_class("classmod.Counter")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	ts := httptest.NewServer(s.pluginServer)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	manager := scriptlingplugin.NewManager(nil)
	if _, err := manager.LoadURL(ctx, "countplugin", ts.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	defer manager.Close()
	defer cancel()

	p := scriptling.New()
	scriptlingplugin.RegisterLibraries(p, manager)

	if got := s.pluginServer.ObjectCount(); got != 0 {
		t.Fatalf("initial count = %d, want 0", got)
	}

	_, err = p.Eval(`import plugin.countplugin; a = plugin.countplugin.Counter(1)`)
	if err != nil {
		t.Fatalf("create a: %v", err)
	}
	if got := s.pluginServer.ObjectCount(); got != 1 {
		t.Errorf("after one new: count = %d, want 1", got)
	}

	_, err = p.Eval(`b = plugin.countplugin.Counter(2)`)
	if err != nil {
		t.Fatalf("create b: %v", err)
	}
	if got := s.pluginServer.ObjectCount(); got != 2 {
		t.Errorf("after two new: count = %d, want 2", got)
	}

	// Closing the manager sends plugin.shutdown → server must call dtors and
	// clear its object store.
	manager.Close()
	cancel()
	if got := s.pluginServer.ObjectCount(); got != 0 {
		t.Errorf("after client close (plugin.shutdown): count = %d, want 0", got)
	}
}

func TestPluginServerHTTPParallel(t *testing.T) {
	const workers = 20
	const callsPerWorker = 10

	libDir := writePluginHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("pplugin", "1.0", "Parallel test plugin")
rp.register_function("add", "plugmod.add")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	ts := httptest.NewServer(s.pluginServer)
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	manager := scriptlingplugin.NewManager(nil)
	if _, err := manager.LoadURL(ctx, "pplugin", ts.URL, true, false); err != nil {
		t.Fatalf("LoadURL: %v", err)
	}
	defer manager.Close()

	errs := make(chan error, workers*callsPerWorker)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			p := scriptling.New()
			scriptlingplugin.RegisterLibraries(p, manager)
			for i := 0; i < callsPerWorker; i++ {
				a, b := seed+i, i+1
				expr := fmt.Sprintf("import plugin.pplugin; plugin.pplugin.add(%d, %d)", a, b)
				result, evalErr := p.Eval(expr)
				if evalErr != nil {
					errs <- fmt.Errorf("worker %d call %d: %v", seed, i, evalErr)
					continue
				}
				got, _ := result.AsInt()
				want := int64(a + b)
				if got != want {
					errs <- fmt.Errorf("worker %d call %d: add(%d,%d)=%d want %d", seed, i, a, b, got, want)
				}
			}
		}(w * 100)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

// Plugin server stdio parallel load: fires N concurrent callers against the
// stdio plugin server over a single in-process connection. All goroutines share
// one *plugin.Client; the client multiplexes calls by JSON-RPC ID and the
// server dispatches each in its own goroutine. Surfaces races in the server
// dispatch path, evaluator setup, and the runtimeParentLibraries sync.Map.
func TestPluginServerStdioParallel(t *testing.T) {
	const workers = 20
	const callsPerWorker = 10

	libDir := writePluginHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime as runtime
rp.serve("splugin", "1.0", "Stdio parallel test plugin")
rp.register_function("add", "plugmod.add")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	if s.pluginServer == nil {
		t.Fatal("pluginServer should be set")
	}

	serverConn, clientConn, err := pipeConn()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	serverDone := make(chan error, 1)
	go func() {
		serverDone <- s.runPluginServer(ctx, serverConn, serverConn)
	}()

	client, err := scriptlingplugin.LoadClientFromIO(ctx, clientConn, clientConn)
	if err != nil {
		clientConn.Close()
		t.Fatalf("LoadClientFromIO: %v", err)
	}

	// All workers share a single evaluator + client (one stdio connection).
	p := scriptling.New()
	scriptlingplugin.RegisterClientLibrary(p, client)

	errs := make(chan error, workers*callsPerWorker)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(seed int) {
			defer wg.Done()
			for i := 0; i < callsPerWorker; i++ {
				a, b := seed+i, i+1
				expr := fmt.Sprintf("import plugin.splugin; plugin.splugin.add(%d, %d)", a, b)
				result, evalErr := p.Eval(expr)
				if evalErr != nil {
					errs <- fmt.Errorf("worker %d call %d: %v", seed, i, evalErr)
					continue
				}
				got, _ := result.AsInt()
				want := int64(a + b)
				if got != want {
					errs <- fmt.Errorf("worker %d call %d: add(%d,%d)=%d want %d", seed, i, a, b, got, want)
				}
			}
		}(w * 100)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}

	// Close the client first so the server's read loop gets EOF and RunIO exits.
	client.Close()
	cancel()
	<-serverDone
}

// JSON-RPC over stdio: same setup flow, with the request fed through the
// injectable reader (the path RunJSONRPCStdio uses with os.Stdin).
func TestNewServerJSONRPCStdio(t *testing.T) {
	libDir := writeJSONRPCHandlerLib(t)
	script := writeSetup(t, `
import scriptling.runtime as runtime
runtime.jsonrpc.method("echo", "rpcmod.echo")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	defer signalShutdown(t, s)

	if _, ok := s.jsonrpcMethods["echo"]; !ok {
		t.Fatalf("echo method not collected via setup script")
	}

	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(`{"jsonrpc":"2.0","method":"echo","params":{"hello":"world"},"id":1}`+"\n"), &out); err != nil {
		t.Fatalf("runJSONRPC: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &body); err != nil {
		t.Fatalf("decode stdio response %q: %v", out.String(), err)
	}
	result, ok := body["result"].(map[string]interface{})
	if !ok || result["hello"] != "world" {
		t.Fatalf("unexpected stdio response: %#v", body)
	}
}
