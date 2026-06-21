package plugin

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

// scriptlingHelper speaks the plugin protocol (handshake + function.call). It
// backs the scriptling=True tests.
func scriptlingHelper() {
	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })

	addFn := object.NewFunctionBuilder()
	addFn.Function(func(a, b int) int { return a + b })

	server := NewServer("declared", "2.3.4", "load test plugin")
	server.RegisterFunc("echo", echoFn)
	server.RegisterFunc("add", addFn)
	_ = server.Run()
	os.Exit(0)
}

func writeScriptlingHelper(t *testing.T, path string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_LOAD_HELPER=1\r\n\"" + exe + "\" -test.run=TestLoadHelper --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_LOAD_HELPER=1 exec \"" + exe + "\" -test.run=TestLoadHelper --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

// TestLoadHelper is the re-exec entry point used by writeScriptlingHelper.
func TestLoadHelper(t *testing.T) {
	if os.Getenv("SCRIPTLING_LOAD_HELPER") == "1" {
		scriptlingHelper()
		return
	}
	t.Skip("driver test — only runs under SCRIPTLING_LOAD_HELPER=1")
}

// rawEchoHelper is a minimal JSON-RPC 2.0 peer that does NOT implement the
// plugin protocol handshake. It dispatches ping/echo/add and is used by the
// scriptling=False (raw) tests.
func rawEchoHelper() {
	decoder := json.NewDecoder(bufio.NewReader(os.Stdin))
	encoder := json.NewEncoder(os.Stdout)
	handle := func(msg map[string]any) (map[string]any, bool) {
		id, hasID := msg["id"]
		method, _ := msg["method"].(string)
		params := msg["params"]

		var result any
		var errResp map[string]any
		quit := false
		switch method {
		case "batch_ping":
			result = "batch-pong"
		case "echo":
			result = params
		case "ping":
			result = "pong"
		case "add":
			if arr, ok := params.([]any); ok && len(arr) == 2 {
				a, _ := arr[0].(float64)
				b, _ := arr[1].(float64)
				result = a + b
			} else if m, ok := params.(map[string]any); ok {
				a, _ := m["a"].(float64)
				b, _ := m["b"].(float64)
				result = a + b
			} else {
				errResp = map[string]any{"code": -32602, "message": "invalid params"}
			}
		case "shutdown", "plugin.shutdown":
			result = nil
			quit = true
		default:
			errResp = map[string]any{"code": -32601, "message": "method not found: " + method}
		}

		if !hasID {
			return nil, quit
		}
		resp := map[string]any{"jsonrpc": "2.0", "id": id}
		if errResp != nil {
			resp["error"] = errResp
		} else {
			resp["result"] = result
		}
		return resp, quit
	}
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			return
		}
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 {
			continue
		}
		if raw[0] == '[' {
			var batch []map[string]any
			if err := json.Unmarshal(raw, &batch); err != nil {
				return
			}
			responses := make([]map[string]any, 0, len(batch))
			quit := false
			for _, msg := range batch {
				resp, stop := handle(msg)
				if resp != nil {
					responses = append(responses, resp)
				}
				quit = quit || stop
			}
			if len(responses) > 0 {
				_ = encoder.Encode(responses)
			}
			if quit {
				return
			}
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal(raw, &msg); err != nil {
			return
		}
		resp, quit := handle(msg)
		if resp != nil {
			_ = encoder.Encode(resp)
		}
		if quit {
			return
		}
	}
}

func writeRawEchoHelper(t *testing.T, path string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var script string
	if runtime.GOOS == "windows" {
		script = "@echo off\r\nset SCRIPTLING_RAW_ECHO_HELPER=1\r\n\"" + exe + "\" -test.run=TestRawEchoHelper --\r\n"
	} else {
		script = "#!/bin/sh\nSCRIPTLING_RAW_ECHO_HELPER=1 exec \"" + exe + "\" -test.run=TestRawEchoHelper --\n"
	}
	if err := os.WriteFile(path, []byte(script), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

func TestRawEchoHelper(t *testing.T) {
	if os.Getenv("SCRIPTLING_RAW_ECHO_HELPER") == "1" {
		rawEchoHelper()
		return
	}
	t.Skip("driver test — only runs under SCRIPTLING_RAW_ECHO_HELPER=1")
}

// --- Go-level tests --------------------------------------------------------

func TestLoadPathScriptlingMode(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	manager := NewManager(nil)
	defer manager.Close()

	client, err := manager.LoadPath(ctx, "loaded", helper, true, nil)
	if err != nil {
		t.Fatalf("LoadPath: %v", err)
	}
	if name := client.Metadata().Name; name != "plugin.loaded" {
		t.Fatalf("expected plugin.loaded, got %q", name)
	}
	// Version still comes from the handshake even though name was overridden.
	if v := client.Metadata().Version; v != "2.3.4" {
		t.Fatalf("expected version 2.3.4 from handshake, got %q", v)
	}

	result, err := client.CallFunction(ctx, "add", []Value{
		{Type: valueInt, Value: int64(20)},
		{Type: valueInt, Value: int64(22)},
	}, nil)
	if err != nil {
		t.Fatalf("CallFunction: %v", err)
	}
	if result.Type != valueInt || numberToInt64(result.Value) != 42 {
		t.Fatalf("expected int 42, got %#v", result)
	}
}

func TestLoadPathSkipHandshake(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	manager := NewManager(nil)
	defer manager.Close()

	// scriptling=False skips the handshake but the executable still answers
	// function.call, so CallFunction works. No metadata is collected.
	client, err := manager.LoadPath(ctx, "skipped", helper, false, nil)
	if err != nil {
		t.Fatalf("LoadPath: %v", err)
	}
	if name := client.Metadata().Name; name != "plugin.skipped" {
		t.Fatalf("expected plugin.skipped, got %q", name)
	}
	if v := client.Metadata().Version; v != "" {
		t.Fatalf("expected empty version when handshake skipped, got %q", v)
	}

	result, err := client.CallFunction(ctx, "add", []Value{
		{Type: valueInt, Value: int64(15)},
		{Type: valueInt, Value: int64(27)},
	}, nil)
	if err != nil {
		t.Fatalf("CallFunction: %v", err)
	}
	if result.Type != valueInt || numberToInt64(result.Value) != 42 {
		t.Fatalf("expected int 42, got %#v", result)
	}
}

func TestLoadPathResolvesBareExecutableWithPATH(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "rawloader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeRawEchoHelper(t, helper)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	manager := NewManager(nil)
	defer manager.Close()

	client, err := manager.LoadPath(ctx, "pathraw", filepath.Base(helper), false, nil)
	if err != nil {
		t.Fatalf("LoadPath: %v", err)
	}
	if client.Path() != helper {
		t.Fatalf("expected resolved path %q, got %q", helper, client.Path())
	}
	var result string
	if err := client.Call(ctx, "ping", nil, &result); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result != "pong" {
		t.Fatalf("expected pong, got %q", result)
	}
}

func TestLoadPathIdempotent(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	manager := NewManager(nil)
	defer manager.Close()

	c1, err := manager.LoadPath(ctx, "loaded", helper, true, nil)
	if err != nil {
		t.Fatalf("first LoadPath: %v", err)
	}
	// Same name + same path is a no-op (scriptling flag is ignored).
	c2, err := manager.LoadPath(ctx, "loaded", helper, false, nil)
	if err != nil {
		t.Fatalf("second LoadPath: %v", err)
	}
	if c1 != c2 {
		t.Fatal("expected same client instance for same name+path")
	}
	if clients := manager.List(); len(clients) != 1 {
		t.Fatalf("expected one client, got %d", len(clients))
	}
}

func TestLoadPathNameMismatchError(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	manager := NewManager(nil)
	defer manager.Close()

	if _, err := manager.LoadPath(ctx, "alpha", helper, true, nil); err != nil {
		t.Fatalf("first LoadPath: %v", err)
	}
	// Same path, different name — must error regardless of scriptling flag.
	_, err := manager.LoadPath(ctx, "beta", helper, true, nil)
	if err == nil {
		t.Fatal("expected error loading same path under a different name")
	}
	if !strings.Contains(err.Error(), "already loaded as plugin.alpha") {
		t.Fatalf("expected 'already loaded as plugin.alpha' error, got %v", err)
	}
}

func TestLoadPathNameCollision(t *testing.T) {
	dir := t.TempDir()
	helper1 := filepath.Join(dir, "loader-a")
	helper2 := filepath.Join(dir, "loader-b")
	if runtime.GOOS == "windows" {
		helper1 += ".bat"
		helper2 += ".bat"
	}
	writeScriptlingHelper(t, helper1)
	writeScriptlingHelper(t, helper2)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	manager := NewManager(nil)
	defer manager.Close()

	if _, err := manager.LoadPath(ctx, "shared", helper1, true, nil); err != nil {
		t.Fatalf("first LoadPath: %v", err)
	}
	// Different path, same name — must error.
	_, err := manager.LoadPath(ctx, "shared", helper2, true, nil)
	if err == nil {
		t.Fatal("expected error loading new path under an existing name")
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Fatalf("expected 'already in use' error, got %v", err)
	}
}

func TestUnload(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	manager := NewManager(nil)
	defer manager.Close()

	if _, err := manager.LoadPath(ctx, "temp", helper, true, nil); err != nil {
		t.Fatalf("LoadPath: %v", err)
	}
	if _, ok := manager.Get("temp"); !ok {
		t.Fatal("expected client before unload")
	}
	if err := manager.Unload("temp"); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	if _, ok := manager.Get("temp"); ok {
		t.Fatal("expected client to be gone after unload")
	}
	if err := manager.Unload("temp"); err == nil {
		t.Fatal("expected error unloading unknown name")
	}
	// After unload, the same name+path can be loaded again.
	if _, err := manager.LoadPath(ctx, "temp", helper, true, nil); err != nil {
		t.Fatalf("reload after unload: %v", err)
	}
}

// TestCloseToleratesMissingShutdown verifies that Close() does not error when
// the peer does not implement plugin.shutdown — essential for raw-mode peers.
func TestCloseToleratesMissingShutdown(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "raw")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeRawEchoHelper(t, helper)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client, err := SpawnClient(ctx, helper, nil)
	if err != nil {
		t.Fatalf("SpawnClient: %v", err)
	}
	var result string
	if err := client.Call(ctx, "ping", nil, &result); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if result != "pong" {
		t.Fatalf("expected pong, got %q", result)
	}
	// rawEchoHelper recognises "shutdown" and exits cleanly; this proves the
	// best-effort path works even though plugin.shutdown itself is not handled.
	if err := client.Close(); err != nil {
		t.Fatalf("Close on raw peer returned error: %v", err)
	}
}

// --- Script-level tests ----------------------------------------------------

func TestControlLibraryScriptlingMode(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	t.Run("load_then_call_function", func(t *testing.T) {
		result, err := p.Eval(`
import scriptling.plugin
name = scriptling.plugin.load("loaded", ` + strconv.Quote(helper) + `, scriptling=True)
scriptling.plugin.call_function(name, "add", 18, 24)
`)
		if err != nil {
			t.Fatalf("Eval: %v", err)
		}
		if i, ok := result.(*object.Integer); !ok || i.IntValue() != 42 {
			t.Fatalf("expected int 42, got %#v", result)
		}
	})

	t.Run("name_normalised", func(t *testing.T) {
		// Same name+path is idempotent; the returned name is still normalised.
		result, err := p.Eval(`
import scriptling.plugin
scriptling.plugin.load("loaded", ` + strconv.Quote(helper) + `, scriptling=True)
`)
		if err != nil {
			t.Fatalf("Eval: %v", err)
		}
		if s, ok := result.(*object.String); !ok || s.StringValue() != "plugin.loaded" {
			t.Fatalf("expected plugin.loaded, got %#v", result)
		}
	})

	t.Run("describe_returns_handshake_metadata", func(t *testing.T) {
		result, err := p.Eval(`
import scriptling.plugin
scriptling.plugin.describe("loaded")["version"]
`)
		if err != nil {
			t.Fatalf("Eval: %v", err)
		}
		if s, ok := result.(*object.String); !ok || s.StringValue() != "2.3.4" {
			t.Fatalf("expected 2.3.4 from handshake, got %#v", result)
		}
	})
}

func TestControlLibraryScriptlingModeRegistersAndUnregistersProxy(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import scriptling.plugin
name = scriptling.plugin.load("dyn", ` + strconv.Quote(helper) + `, scriptling=True)

import plugin.dyn
before = plugin.dyn.echo("hello")

scriptling.plugin.unload(name)

missing_import = False
try:
    import plugin.dyn
except ImportError:
    missing_import = True

missing_proxy = False
try:
    plugin.dyn.echo("again")
except Exception:
    missing_proxy = True

[before, missing_import, missing_proxy, scriptling.plugin.list()]
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 4 {
		t.Fatalf("expected 4-item list, got %#v", result)
	}
	if s, ok := list.Elements[0].(*object.String); !ok || s.StringValue() != "hello" {
		t.Fatalf("expected proxy call result hello, got %#v", list.Elements[0])
	}
	if b, ok := list.Elements[1].(*object.Boolean); !ok || !b.BoolValue() {
		t.Fatalf("expected import to fail after unload, got %#v", list.Elements[1])
	}
	if b, ok := list.Elements[2].(*object.Boolean); !ok || !b.BoolValue() {
		t.Fatalf("expected proxy binding to be removed after unload, got %#v", list.Elements[2])
	}
	if loaded, ok := list.Elements[3].(*object.List); !ok || len(loaded.Elements) != 0 {
		t.Fatalf("expected plugin list to be empty after unload, got %#v", list.Elements[3])
	}
}

func TestControlLibraryDescribeWithoutHandshake(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	// scriptling=False skips the handshake, so describe() reports no version,
	// but the host still knows the stdio codec is JSON.
	result, err := p.Eval(`
import scriptling.plugin
scriptling.plugin.load("skipped", ` + strconv.Quote(helper) + `)
scriptling.plugin.describe("skipped")["version"]
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if s, ok := result.(*object.String); ok {
		if s.StringValue() != "" {
			t.Fatalf("expected empty version without handshake, got %q", s.StringValue())
		}
	} else if _, ok := result.(*object.Null); !ok {
		t.Fatalf("expected empty string or null, got %#v", result)
	}

	result, err = p.Eval(`
import scriptling.plugin
scriptling.plugin.describe("skipped")["transport"]
`)
	if err != nil {
		t.Fatalf("Eval transport: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "json" {
		t.Fatalf("expected json transport without handshake, got %#v", result)
	}
}

func TestControlLibraryLoadIdempotent(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import scriptling.plugin
a = scriptling.plugin.load("idem", ` + strconv.Quote(helper) + `, scriptling=True)
b = scriptling.plugin.load("idem", ` + strconv.Quote(helper) + `, scriptling=True)
a == b
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if b, ok := result.(*object.Boolean); !ok || !b.BoolValue() {
		t.Fatalf("expected second load to return same name, got %#v", result)
	}
}

func TestControlLibraryLoadNameMismatch(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	_, err := p.Eval(`
import scriptling.plugin
scriptling.plugin.load("alpha", ` + strconv.Quote(helper) + `, scriptling=True)
scriptling.plugin.load("beta", ` + strconv.Quote(helper) + `, scriptling=True)
`)
	if err == nil {
		t.Fatal("expected error loading same path under different name")
	}
	if !strings.Contains(err.Error(), "already loaded as") {
		t.Fatalf("expected 'already loaded as' error, got %v", err)
	}
}

func TestControlLibraryUnload(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	_, err := p.Eval(`
import scriptling.plugin
name = scriptling.plugin.load("tounload", ` + strconv.Quote(helper) + `, scriptling=True)
scriptling.plugin.unload(name)
scriptling.plugin.call_function(name, "echo", "x")
`)
	if err == nil {
		t.Fatal("expected error calling after unload")
	}
	if !strings.Contains(err.Error(), "plugin not found") {
		t.Fatalf("expected 'plugin not found' error, got %v", err)
	}
}

func TestControlLibraryLoadURLScriptlingMode(t *testing.T) {
	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })

	server := NewServer("httpdeclared", "2.3.4", "http load test plugin")
	server.RegisterFunc("echo", echoFn)
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import scriptling.plugin
name = scriptling.plugin.load("httppeer", ` + strconv.Quote(httpServer.URL) + `, scriptling=True)
import plugin.httppeer
value = plugin.httppeer.echo("http")
scriptling.plugin.unload(name)

missing_import = False
try:
    import plugin.httppeer
except ImportError:
    missing_import = True

[value, missing_import]
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 2 {
		t.Fatalf("expected 2-item result, got %#v", result)
	}
	if s, ok := list.Elements[0].(*object.String); !ok || s.StringValue() != "http" {
		t.Fatalf("expected http result, got %#v", list.Elements[0])
	}
	if b, ok := list.Elements[1].(*object.Boolean); !ok || !b.BoolValue() {
		t.Fatalf("expected HTTP proxy import to fail after unload, got %#v", list.Elements[1])
	}
}

func TestControlLibraryLoadURLHeaders(t *testing.T) {
	const token = "Bearer plugin-load-test"

	echoFn := object.NewFunctionBuilder()
	echoFn.Function(func(v any) any { return v })

	server := NewServer("httpheaders", "2.3.4", "http headers test plugin")
	server.RegisterFunc("echo", echoFn)

	var requests int
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if got := r.Header.Get("Authorization"); got != token {
			t.Errorf("expected Authorization header %q, got %q", token, got)
		}
		if got := r.Header.Get("X-Scriptling-Test"); got != "headers" {
			t.Errorf("expected X-Scriptling-Test header headers, got %q", got)
		}
		server.ServeHTTP(w, r)
	}))
	defer httpServer.Close()

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import scriptling.plugin
name = scriptling.plugin.load(
    "httpheaders",
    ` + strconv.Quote(httpServer.URL) + `,
    scriptling=True,
    headers={"Authorization": ` + strconv.Quote(token) + `, "X-Scriptling-Test": "headers"},
)
import plugin.httpheaders
plugin.httpheaders.echo("ok")
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "ok" {
		t.Fatalf("expected ok result, got %#v", result)
	}
	if requests < 2 {
		t.Fatalf("expected headers on handshake and call requests, got %d request(s)", requests)
	}
}

func TestControlLibraryCallFunctionAutoRoutesRaw(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "raw")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeRawEchoHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	// No scriptling=True, so no handshake. call_function must detect this and
	// send the method name directly (raw JSON-RPC) rather than wrapping it in
	// function.call. This is what makes --json-rpc peers work.
	result, err := p.Eval(`
import scriptling.plugin
name = scriptling.plugin.load("rawpeer", ` + strconv.Quote(helper) + `)
scriptling.plugin.call_function(name, "ping")
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "pong" {
		t.Fatalf("expected pong via auto-routed call_function, got %#v", result)
	}

	// Single dict positional arg → becomes JSON-RPC params object directly.
	result, err = p.Eval(`
import scriptling.plugin
scriptling.plugin.call_function("rawpeer", "add", {"a": 100, "b": 23})
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	switch v := result.(type) {
	case *object.Integer:
		if v.IntValue() != 123 {
			t.Fatalf("expected 123, got %d", v.IntValue())
		}
	case *object.Float:
		if v.FloatValue() != 123 {
			t.Fatalf("expected 123, got %v", v.FloatValue())
		}
	default:
		t.Fatalf("expected numeric result, got %#v", result)
	}
}

func TestControlLibraryRawBatchResponse(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "raw")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeRawEchoHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import scriptling.plugin
name = scriptling.plugin.load("rawbatch", ` + strconv.Quote(helper) + `)
scriptling.plugin.call_function(name, "batch_ping")
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if s, ok := result.(*object.String); !ok || s.StringValue() != "batch-pong" {
		t.Fatalf("expected batch-pong via batched response, got %#v", result)
	}
}

func TestControlLibraryBatchCallRaw(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "raw")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeRawEchoHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import scriptling.plugin
name = scriptling.plugin.load("rawbatchcall", ` + strconv.Quote(helper) + `)
scriptling.plugin.batch_call(name, [
    {"name": "ping"},
    {"name": "add", "args": [20, 22]},
    {"name": "add", "kwargs": {"a": 5, "b": 7}},
])
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 3 {
		t.Fatalf("expected 3-item list, got %#v", result)
	}
	if s, ok := list.Elements[0].(*object.String); !ok || s.StringValue() != "pong" {
		t.Fatalf("expected pong at index 0, got %#v", list.Elements[0])
	}
	switch v := list.Elements[1].(type) {
	case *object.Integer:
		if v.IntValue() != 42 {
			t.Fatalf("expected 42 at index 1, got %d", v.IntValue())
		}
	case *object.Float:
		if v.FloatValue() != 42 {
			t.Fatalf("expected 42 at index 1, got %v", v.FloatValue())
		}
	default:
		t.Fatalf("expected numeric result at index 1, got %#v", list.Elements[1])
	}
	switch v := list.Elements[2].(type) {
	case *object.Integer:
		if v.IntValue() != 12 {
			t.Fatalf("expected 12 at index 2, got %d", v.IntValue())
		}
	case *object.Float:
		if v.FloatValue() != 12 {
			t.Fatalf("expected 12 at index 2, got %v", v.FloatValue())
		}
	default:
		t.Fatalf("expected numeric result at index 2, got %#v", list.Elements[2])
	}
}

func TestControlLibraryBatchCallScriptling(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import scriptling.plugin
name = scriptling.plugin.load("scriptbatchcall", ` + strconv.Quote(helper) + `, scriptling=True)
scriptling.plugin.batch_call(name, [
    {"name": "add", "args": [19, 23]},
    {"name": "echo", "args": ["done"]},
])
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	list, ok := result.(*object.List)
	if !ok || len(list.Elements) != 2 {
		t.Fatalf("expected 2-item list, got %#v", result)
	}
	if i, ok := list.Elements[0].(*object.Integer); !ok || i.IntValue() != 42 {
		t.Fatalf("expected int 42 at index 0, got %#v", list.Elements[0])
	}
	if s, ok := list.Elements[1].(*object.String); !ok || s.StringValue() != "done" {
		t.Fatalf("expected done at index 1, got %#v", list.Elements[1])
	}
}

func TestControlLibraryShortName(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	// load() returns the normalised name, but call_function / describe / unload
	// all accept the short name too (normalised internally by Manager.Get).
	result, err := p.Eval(`
import scriptling.plugin
scriptling.plugin.load("widgets", ` + strconv.Quote(helper) + `, scriptling=True)
scriptling.plugin.call_function("widgets", "add", 20, 22)
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if i, ok := result.(*object.Integer); !ok || i.IntValue() != 42 {
		t.Fatalf("expected int 42 via short name, got %#v", result)
	}
}

func TestControlLibraryLoadWithArgs(t *testing.T) {
	dir := t.TempDir()
	// A wrapper that only execs the test binary when --plugin-test is present
	// in its arguments. Without that arg it exits non-zero so we can prove the
	// args actually reached the executable.
	helper := filepath.Join(dir, "argcheck")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	var wrapper string
	if runtime.GOOS == "windows" {
		wrapper = "@echo off\r\nset FOUND=0\r\nfor %%a in (%*) do if \"%%a\"==\"--plugin-test\" set FOUND=1\r\nif not \"%FOUND%\"==\"1\" exit 1\r\nset SCRIPTLING_LOAD_HELPER=1\r\n\"" + exe + "\" -test.run=TestLoadHelper --\r\n"
	} else {
		wrapper = "#!/bin/sh\nfor arg in \"$@\"; do\n  if [ \"$arg\" = \"--plugin-test\" ]; then\n    SCRIPTLING_LOAD_HELPER=1 exec \"" + exe + "\" -test.run=TestLoadHelper --\n  fi\ndone\nexit 1\n"
	}
	if err := os.WriteFile(helper, []byte(wrapper), 0755); err != nil {
		t.Fatalf("write helper: %v", err)
	}

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	t.Run("without_args_fails", func(t *testing.T) {
		_, err := p.Eval(`
import scriptling.plugin
scriptling.plugin.load("noargs", ` + strconv.Quote(helper) + `, scriptling=True)
`)
		if err == nil {
			t.Fatal("expected error when required args are missing")
		}
	})

	t.Run("with_args_succeeds", func(t *testing.T) {
		result, err := p.Eval(`
import scriptling.plugin
name = scriptling.plugin.load("withargs", ` + strconv.Quote(helper) + `, scriptling=True, args=["--plugin-test"])
scriptling.plugin.call_function(name, "add", 19, 23)
`)
		if err != nil {
			t.Fatalf("Eval: %v", err)
		}
		if i, ok := result.(*object.Integer); !ok || i.IntValue() != 42 {
			t.Fatalf("expected int 42, got %#v", result)
		}
	})
}

func TestControlLibraryListIncludesLoaded(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	p := scriptling.New()
	RegisterLibraries(p, manager)

	result, err := p.Eval(`
import scriptling.plugin
scriptling.plugin.load("listed", ` + strconv.Quote(helper) + `, scriptling=True)
any(m["name"] == "plugin.listed" for m in scriptling.plugin.list())
`)
	if err != nil {
		t.Fatalf("Eval: %v", err)
	}
	if b, ok := result.(*object.Boolean); !ok || !b.BoolValue() {
		t.Fatalf("expected True, got %#v", result)
	}
}

// TestControlLibraryLoadConcurrent ensures overlapping load() calls from
// separate Scriptling environments on the same shared manager don't race and
// produce exactly one client per name+path.
func TestControlLibraryLoadConcurrent(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "loader")
	if runtime.GOOS == "windows" {
		helper += ".bat"
	}
	writeScriptlingHelper(t, helper)

	manager := NewManager(nil)
	defer manager.Close()

	const goroutines = 8
	var wg sync.WaitGroup
	var failures atomic.Int64
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			p := scriptling.New()
			RegisterLibraries(p, manager)
			_, err := p.Eval(fmt.Sprintf(`
import scriptling.plugin
name = scriptling.plugin.load("concurrent", %q, scriptling=True)
scriptling.plugin.call_function(name, "echo", %d)
`, helper, id))
			if err != nil {
				t.Logf("goroutine %d: %v", id, err)
				failures.Add(1)
				return
			}
		}(i)
	}
	wg.Wait()

	if f := failures.Load(); f > 0 {
		t.Fatalf("%d concurrent goroutines failed", f)
	}
	if clients := manager.List(); len(clients) != 1 {
		t.Fatalf("expected exactly one client after concurrent loads, got %d", len(clients))
	}
}
