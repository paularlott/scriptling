package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs"
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
