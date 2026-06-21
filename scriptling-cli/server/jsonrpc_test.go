package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

// jsonrpcTestServer builds a Server wired for JSON-RPC tests: a temp lib dir
// holds the handler module, and the given methods map is installed directly.
func jsonrpcTestServer(t *testing.T, handlerSrc string, methods, notifications map[string]string) (*Server, string) {
	t.Helper()

	libDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(libDir, "rpcmod.py"), []byte(handlerSrc), 0644); err != nil {
		t.Fatal(err)
	}

	// One-time global factory configuration (idempotent enough for tests).
	setup.Factories([]string{libDir}, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "")

	extlibs.ResetRuntime()

	s := &Server{
		config: ServerConfig{
			LibDirs: []string{libDir},
		},
		jsonrpcMethods:       make(map[string]string),
		jsonrpcNotifications: make(map[string]string),
	}
	for name, ref := range methods {
		s.jsonrpcMethods[name] = ref
	}
	for name, ref := range notifications {
		s.jsonrpcNotifications[name] = ref
	}
	return s, libDir
}

func TestJSONRPCHTTPSingleRequest(t *testing.T) {
	src := `def echo(params):
    return params
`
	s, _ := jsonrpcTestServer(t, src, map[string]string{"echo": "rpcmod.echo"}, nil)
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
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("expected json content type, got %q", ct)
	}
	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result, ok := body["result"].(map[string]interface{})
	if !ok || result["hello"] != "world" {
		t.Fatalf("unexpected result: %#v", body)
	}
}

func TestJSONRPCHTTPBatch(t *testing.T) {
	src := `def echo(params):
    return params

def on_event(params):
    return None
`
	s, _ := jsonrpcTestServer(t, src,
		map[string]string{"echo": "rpcmod.echo"},
		map[string]string{"event": "rpcmod.on_event"},
	)
	httpSrv := httptest.NewServer(http.HandlerFunc(s.handleJSONRPCHTTP))
	defer httpSrv.Close()

	body := `[
		{"jsonrpc":"2.0","method":"echo","params":{"n":1},"id":1},
		{"jsonrpc":"2.0","method":"event","params":{"x":99}},
		{"jsonrpc":"2.0","method":"echo","params":{"n":2},"id":2}
	]`
	resp, err := http.Post(httpSrv.URL, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatalf("POST /json-rpc: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var arr []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&arr); err != nil {
		t.Fatalf("decode batch: %v", err)
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 request responses, got %d: %#v", len(arr), arr)
	}
	ids := map[int]bool{}
	for _, r := range arr {
		ids[asInt(r["id"])] = true
	}
	if !ids[1] || !ids[2] {
		t.Fatalf("expected ids 1 and 2, got %v", ids)
	}
}

func TestJSONRPCHTTPNotificationNoContent(t *testing.T) {
	src := `def on_note(params):
    return None
`
	s, _ := jsonrpcTestServer(t, src, nil, map[string]string{"noted": "rpcmod.on_note"})
	httpSrv := httptest.NewServer(http.HandlerFunc(s.handleJSONRPCHTTP))
	defer httpSrv.Close()

	resp, err := http.Post(httpSrv.URL, "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"noted","params":{"x":1}}`))
	if err != nil {
		t.Fatalf("POST /json-rpc: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty body, got %q", string(body))
	}
}

func TestJSONRPCHTTPRejectsNonPOST(t *testing.T) {
	s, _ := jsonrpcTestServer(t, "def noop(params):\n    return None\n", nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/json-rpc", nil)
	rec := httptest.NewRecorder()

	s.handleJSONRPCHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != http.MethodPost {
		t.Fatalf("expected Allow POST, got %q", allow)
	}
}

// readJSONRPCLines splits NDJSON output into decoded response objects. It uses
// UseNumber so large integer ids/results keep full precision.
func readJSONRPCLines(t *testing.T, out []byte) []map[string]interface{} {
	t.Helper()
	var res []map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.UseNumber()
	for {
		var m map[string]interface{}
		if err := dec.Decode(&m); err != nil {
			break
		}
		res = append(res, m)
	}
	return res
}

// asInt extracts an int from a decoded JSON value (json.Number or float64).
func asInt(v interface{}) int {
	if n, ok := v.(json.Number); ok {
		i, _ := n.Int64()
		return int(i)
	}
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

func TestJSONRPCSingleRequest(t *testing.T) {
	src := `def echo(params):
    return params
`
	s, _ := jsonrpcTestServer(t, src, map[string]string{"echo": "rpcmod.echo"}, nil)

	input := `{"jsonrpc":"2.0","method":"echo","params":{"hello":"world"},"id":1}`
	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("runJSONRPC failed: %v", err)
	}

	lines := readJSONRPCLines(t, out.Bytes())
	if len(lines) != 1 {
		t.Fatalf("expected 1 response, got %d", len(lines))
	}
	resp := lines[0]
	if resp["jsonrpc"] != "2.0" {
		t.Errorf("expected jsonrpc 2.0, got %v", resp["jsonrpc"])
	}
	if id := asInt(resp["id"]); id != 1 {
		t.Errorf("expected id 1, got %v", resp["id"])
	}
	params, ok := resp["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected result map, got %T: %v", resp["result"], resp["result"])
	}
	if params["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", params["hello"])
	}
	if _, present := resp["error"]; present {
		t.Errorf("did not expect error in response: %v", resp["error"])
	}
}

func TestJSONRPCNotificationNoResponse(t *testing.T) {
	src := `def on_note(params):
    return params
`
	s, _ := jsonrpcTestServer(t, src, nil, map[string]string{"noted": "rpcmod.on_note"})

	// A notification: no "id" field. The server must NOT write any response.
	input := `{"jsonrpc":"2.0","method":"noted","params":{"x":1}}`
	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("runJSONRPC failed: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for notification, got %q", out.String())
	}
}

func TestJSONRPCMethodNotFound(t *testing.T) {
	s, _ := jsonrpcTestServer(t, "def noop(params):\n    return None\n", nil, nil)

	input := `{"jsonrpc":"2.0","method":"missing","id":42}`
	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("runJSONRPC failed: %v", err)
	}

	lines := readJSONRPCLines(t, out.Bytes())
	if len(lines) != 1 {
		t.Fatalf("expected 1 response, got %d", len(lines))
	}
	errObj, ok := lines[0]["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected error object, got %v", lines[0])
	}
	code := asInt(errObj["code"])
	if int(code) != jsonrpcMethodNotFound {
		t.Errorf("expected code %d, got %v", jsonrpcMethodNotFound, errObj["code"])
	}
}

func TestJSONRPCHandlerExceptionBecomesServerError(t *testing.T) {
	src := `def boom(params):
    raise ValueError("kaboom")
`
	s, _ := jsonrpcTestServer(t, src, map[string]string{"boom": "rpcmod.boom"}, nil)

	input := `{"jsonrpc":"2.0","method":"boom","id":7}`
	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("runJSONRPC failed: %v", err)
	}

	lines := readJSONRPCLines(t, out.Bytes())
	if len(lines) != 1 {
		t.Fatalf("expected 1 response, got %d", len(lines))
	}
	errObj, _ := lines[0]["error"].(map[string]interface{})
	code := asInt(errObj["code"])
	if int(code) != jsonrpcServerError {
		t.Errorf("expected code %d, got %v", jsonrpcServerError, errObj["code"])
	}
	if !strings.Contains(errObj["message"].(string), "kaboom") {
		t.Errorf("expected message to contain 'kaboom', got %v", errObj["message"])
	}
}

func TestJSONRPCCustomErrorViaHelper(t *testing.T) {
	src := `import scriptling.runtime as runtime

def divide(params):
    if params["b"] == 0:
        return runtime.jsonrpc.error(-32602, "division by zero", {"field": "b"})
    return params["a"] / params["b"]
`
	s, _ := jsonrpcTestServer(t, src, map[string]string{"divide": "rpcmod.divide"}, nil)

	input := `{"jsonrpc":"2.0","method":"divide","params":{"a":10,"b":0},"id":"req-1"}`
	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("runJSONRPC failed: %v", err)
	}

	lines := readJSONRPCLines(t, out.Bytes())
	if len(lines) != 1 {
		t.Fatalf("expected 1 response, got %d", len(lines))
	}
	// String id round-trips exactly.
	if lines[0]["id"] != "req-1" {
		t.Errorf("expected id 'req-1', got %v", lines[0]["id"])
	}
	errObj, _ := lines[0]["error"].(map[string]interface{})
	code := asInt(errObj["code"])
	if int(code) != -32602 {
		t.Errorf("expected code -32602, got %v", errObj["code"])
	}
	if errObj["message"] != "division by zero" {
		t.Errorf("expected message, got %v", errObj["message"])
	}
	data, _ := errObj["data"].(map[string]interface{})
	if data["field"] != "b" {
		t.Errorf("expected data.field=b, got %v", errObj["data"])
	}
}

func TestJSONRPCBatch(t *testing.T) {
	src := `def echo(params):
    return params
`
	s, _ := jsonrpcTestServer(t, src, map[string]string{"echo": "rpcmod.echo"}, nil)

	input := `[
		{"jsonrpc":"2.0","method":"echo","params":{"n":1},"id":1},
		{"jsonrpc":"2.0","method":"echo","params":{"n":2},"id":2}
	]`
	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("runJSONRPC failed: %v", err)
	}

	// Batch response is a single JSON array.
	var arr []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &arr); err != nil {
		t.Fatalf("expected batch array, got parse error: %v (output: %q)", err, out.String())
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 batch responses, got %d", len(arr))
	}
	// Order is not guaranteed (concurrent), so collect ids.
	ids := map[int]bool{}
	for _, r := range arr {
		ids[asInt(r["id"])] = true
	}
	if !ids[1] || !ids[2] {
		t.Errorf("expected ids 1 and 2 in batch responses, got %v", ids)
	}
}

func TestJSONRPCBatchAllNotificationsNoResponse(t *testing.T) {
	src := `def on_event(params):
    return None
`
	s, _ := jsonrpcTestServer(t, src, nil, map[string]string{"event": "rpcmod.on_event"})

	input := `[
		{"jsonrpc":"2.0","method":"event","params":{"x":1}},
		{"jsonrpc":"2.0","method":"event","params":{"x":2}}
	]`
	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("runJSONRPC failed: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no output for all-notification batch, got %q", out.String())
	}
}

func TestJSONRPCBatchMixedRequestsAndNotifications(t *testing.T) {
	src := `def echo(params):
    return params

def on_event(params):
    return None
`
	s, _ := jsonrpcTestServer(t, src,
		map[string]string{"echo": "rpcmod.echo"},
		map[string]string{"event": "rpcmod.on_event"},
	)

	input := `[
		{"jsonrpc":"2.0","method":"echo","params":{"n":1},"id":1},
		{"jsonrpc":"2.0","method":"event","params":{"x":99}},
		{"jsonrpc":"2.0","method":"echo","params":{"n":2},"id":2}
	]`
	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("runJSONRPC failed: %v", err)
	}

	var arr []map[string]interface{}
	if err := json.Unmarshal(out.Bytes(), &arr); err != nil {
		t.Fatalf("expected batch array, got parse error: %v (output: %q)", err, out.String())
	}
	if len(arr) != 2 {
		t.Fatalf("expected 2 request responses and no notification response, got %d: %#v", len(arr), arr)
	}
	ids := map[int]bool{}
	for _, r := range arr {
		ids[asInt(r["id"])] = true
	}
	if !ids[1] || !ids[2] {
		t.Errorf("expected ids 1 and 2 in mixed batch responses, got %v", ids)
	}
}

// TestJSONRPCConcurrent verifies that handlers run concurrently: two slow
// handlers (each ~150ms) must finish in roughly one handler's time, not the sum.
func TestJSONRPCConcurrent(t *testing.T) {
	src := `import time

def slow_a(params):
    time.sleep(0.15)
    return "a"

def slow_b(params):
    time.sleep(0.15)
    return "b"
`
	s, _ := jsonrpcTestServer(t, src, map[string]string{
		"a": "rpcmod.slow_a",
		"b": "rpcmod.slow_b",
	}, nil)

	// Two requests back-to-back. If serialised they'd take ~300ms; concurrent
	// they take ~150ms. Threshold sits well above concurrent and well below serial.
	input := `{"jsonrpc":"2.0","method":"a","id":1}
{"jsonrpc":"2.0","method":"b","id":2}`
	var out bytes.Buffer
	start := time.Now()
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("runJSONRPC failed: %v", err)
	}
	elapsed := time.Since(start)

	lines := readJSONRPCLines(t, out.Bytes())
	if len(lines) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(lines))
	}

	if elapsed > 250*time.Millisecond {
		t.Errorf("expected concurrent execution (<250ms), took %v", elapsed)
	}
}

func TestJSONRPCNumberPrecisionPreserved(t *testing.T) {
	src := `def echo(params):
    return params
`
	s, _ := jsonrpcTestServer(t, src, map[string]string{"echo": "rpcmod.echo"}, nil)

	// Large integer that exceeds float64 exact range; UseNumber keeps it exact.
	input := `{"jsonrpc":"2.0","method":"echo","params":{"big":9007199254740993},"id":1}`
	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err != nil {
		t.Fatalf("runJSONRPC failed: %v", err)
	}

	lines := readJSONRPCLines(t, out.Bytes())
	if len(lines) != 1 {
		t.Fatalf("expected 1 response, got %d", len(lines))
	}
	result, _ := lines[0]["result"].(map[string]interface{})
	bigNum, ok := result["big"].(json.Number)
	if !ok {
		t.Fatalf("expected json.Number to preserve precision, got %T: %v", result["big"], result["big"])
	}
	big, err := bigNum.Int64()
	if err != nil {
		t.Fatalf("expected int64 conversion, got error: %v", err)
	}
	if big != 9007199254740993 {
		t.Errorf("expected big int preserved, got %d", big)
	}
}

func TestJSONRPCParseError(t *testing.T) {
	s, _ := jsonrpcTestServer(t, "def noop(params):\n    return None\n", nil, nil)

	input := `{not valid json`
	var out bytes.Buffer
	if err := s.runJSONRPC(context.Background(), strings.NewReader(input), &out); err == nil {
		t.Fatal("expected error from malformed input")
	}

	lines := readJSONRPCLines(t, out.Bytes())
	if len(lines) != 1 {
		t.Fatalf("expected 1 parse-error response, got %d", len(lines))
	}
	errObj, _ := lines[0]["error"].(map[string]interface{})
	code := asInt(errObj["code"])
	if int(code) != jsonrpcParseError {
		t.Errorf("expected parse error code %d, got %v", jsonrpcParseError, errObj["code"])
	}
}

// TestRuntimeJSONRPCLibraryRegistration confirms method/notification/error
// populate RuntimeState when invoked from a Scriptling script.
func TestRuntimeJSONRPCLibraryRegistration(t *testing.T) {
	extlibs.ResetRuntime()

	srv, _ := jsonrpcTestServer(t, "", nil, nil)
	p := scriptling.New()
	srv.setupScriptling(p)
	result, err := p.Eval(`
import scriptling.runtime as runtime

runtime.jsonrpc.method("add", "rpcmod.add")
runtime.jsonrpc.notification("ping", "rpcmod.ping")
err = runtime.jsonrpc.error(-32602, "bad")
err
`)
	if err != nil {
		t.Fatal(err)
	}

	if extlibs.RuntimeState.JSONRPCMethods["add"] != "rpcmod.add" {
		t.Errorf("method not registered: %v", extlibs.RuntimeState.JSONRPCMethods)
	}
	if extlibs.RuntimeState.JSONRPCNotifications["ping"] != "rpcmod.ping" {
		t.Errorf("notification not registered: %v", extlibs.RuntimeState.JSONRPCNotifications)
	}

	if !extlibs.IsJSONRPCError(result) {
		t.Errorf("expected JSONRPCError instance, got %T (%v)", result, result)
	}
}
