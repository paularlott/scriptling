package server

// Tests for middleware-to-handler context passing: middleware writes to
// request.context, and every handler kind can read it — HTTP routes via the
// request object, MCP tools via scriptling.mcp.tool.request_context() /
// get_request(), JSON-RPC methods via runtime.jsonrpc.request_context() /
// get_request(), and WebSocket handlers via the tool accessors against the
// upgrade request.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	extlibsmcp "github.com/paularlott/scriptling/extlibs/mcp"
)

// ── HTTP routes ──────────────────────────────────────────────────────────────

// Middleware and route handler share one Request instance, and the tool-side
// accessors see the same stashed request.
func TestMiddlewareContextReachesHTTPHandler(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
def check(request):
    request.context["user"] = "alice"
    return None
`))
	writeFile(t, libDir+"/apimod.py", []byte(`
import scriptling.mcp.tool as tool

def who(request):
    direct = request.context["user"]
    via_accessor = tool.request_context().get("user", "anon")
    rid = tool.get_request().header("x-request-id", "none")
    return {"body": direct + ":" + via_accessor + ":" + rid}
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.route("/api/who", "apimod.who")
http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest("GET", ts.URL+"/api/who", nil)
	req.Header.Set("X-Request-Id", "req-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/who: %v", err)
	}
	defer resp.Body.Close()
	body := make([]byte, 128)
	n, _ := resp.Body.Read(body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := strings.TrimSpace(string(body[:n])); got != "alice:alice:req-123" {
		t.Fatalf("handler context = %q, want alice:alice:req-123", got)
	}
}

// Without a middleware the request is still stashed: get_request() works and
// request_context() is an empty dict.
func TestHTTPRequestAvailableToHandlerWithoutMiddleware(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/apimod.py", []byte(`
import scriptling.mcp.tool as tool

def who(request):
    user = tool.request_context().get("user", "anon")
    rid = tool.get_request().header("x-request-id", "none")
    return {"body": user + ":" + rid}
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.route("/api/who", "apimod.who")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	req, _ := http.NewRequest("GET", ts.URL+"/api/who", nil)
	req.Header.Set("X-Request-Id", "req-456")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/who: %v", err)
	}
	defer resp.Body.Close()
	body := make([]byte, 128)
	n, _ := resp.Body.Read(body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := strings.TrimSpace(string(body[:n])); got != "anon:req-456" {
		t.Fatalf("handler saw %q, want anon:req-456", got)
	}
}

// ── MCP tools ────────────────────────────────────────────────────────────────

// writeContextToolServer builds a serving test server with an optional
// middleware and a "grab" tool that reports the middleware context user and a
// request header.
func writeContextToolServer(t *testing.T, middleware bool) *httptest.Server {
	t.Helper()
	libDir := t.TempDir()
	toolsDir := t.TempDir()

	if middleware {
		writeFile(t, libDir+"/authmod.py", []byte(`
def check(request):
    if request.header("authorization", "") != "Bearer alice-key":
        return {"status": 401, "body": "unauthorized"}
    request.context["user"] = "alice"
    return None
`))
	}
	writeFile(t, toolsDir+"/grab.toml", []byte("description = \"Grab request info\"\n"))
	writeFile(t, toolsDir+"/grab.py", []byte(`
import scriptling.mcp.tool as tool

user = tool.request_context().get("user", "anon")
rid = "none"
req = tool.get_request()
if req != None:
    rid = req.header("x-request-id", "none")
tool.return_string(user + ":" + rid)
`))

	middlewareSetup := ""
	if middleware {
		middlewareSetup = "\nhttp.middleware(\"authmod.check\")"
	}
	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime
`+middlewareSetup+`
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{
		ScriptFile:  script,
		LibDirs:     []string{libDir},
		MCPToolsDir: toolsDir,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts
}

// callGrabTool posts a tools/call for grab with the given headers and returns
// the first content item's text.
func callGrabTool(t *testing.T, ts *httptest.Server, auth string, requestID string) string {
	t.Helper()
	req, err := http.NewRequest("POST", ts.URL+"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"grab","arguments":{}}}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	if requestID != "" {
		req.Header.Set("X-Request-Id", requestID)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var parsed map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result, _ := parsed["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) < 1 {
		t.Fatalf("tool call produced no content: %#v", parsed)
	}
	first, _ := content[0].(map[string]any)
	text, _ := first["text"].(string)
	return text
}

func TestMiddlewareContextReachesMCPTool(t *testing.T) {
	ts := writeContextToolServer(t, true)

	// Without the token the middleware still blocks the call.
	req, _ := http.NewRequest("POST", ts.URL+"/mcp",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"grab","arguments":{}}}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp without token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}

	if got := callGrabTool(t, ts, "Bearer alice-key", "req-mcp"); got != "alice:req-mcp" {
		t.Fatalf("tool saw %q, want alice:req-mcp", got)
	}
}

func TestMCPRequestAvailableToToolWithoutMiddleware(t *testing.T) {
	ts := writeContextToolServer(t, false)

	if got := callGrabTool(t, ts, "", "req-open"); got != "anon:req-open" {
		t.Fatalf("tool saw %q, want anon:req-open", got)
	}
}

// ── JSON-RPC methods ─────────────────────────────────────────────────────────

func TestMiddlewareContextReachesJSONRPCHandler(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
def check(request):
    if request.header("authorization", "") != "Bearer bob-key":
        return {"status": 401, "body": "unauthorized"}
    request.context["user"] = "bob"
    return None
`))
	writeFile(t, libDir+"/rpcmod.py", []byte(`
import scriptling.runtime as runtime

def who(params):
    user = runtime.jsonrpc.request_context().get("user", "anon")
    rid = "none"
    req = runtime.jsonrpc.get_request()
    if req != None:
        rid = req.header("x-request-id", "none")
    return {"user": user, "rid": rid}
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.middleware("authmod.check")
runtime.jsonrpc.method("who", "rpcmod.who")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}, JSONRPC: true})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	post := func(auth, requestID string) (int, map[string]any) {
		req, _ := http.NewRequest("POST", ts.URL+"/json-rpc",
			strings.NewReader(`{"jsonrpc":"2.0","method":"who","params":{},"id":1}`))
		req.Header.Set("Content-Type", "application/json")
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		if requestID != "" {
			req.Header.Set("X-Request-Id", requestID)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("POST /json-rpc: %v", err)
		}
		defer resp.Body.Close()
		var body map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return resp.StatusCode, body
	}

	if status, _ := post("", ""); status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", status)
	}

	status, body := post("Bearer bob-key", "req-jr")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, body)
	}
	result, _ := body["result"].(map[string]any)
	if result["user"] != "bob" || result["rid"] != "req-jr" {
		t.Fatalf("handler saw %#v, want user=bob rid=req-jr", result)
	}
}

// ── Outside a served request (stdio transports) ─────────────────────────────

func TestRequestAccessorsOutsideHTTPRequest(t *testing.T) {
	p := scriptling.New()
	extlibsmcp.RegisterToolHelpers(p)

	if _, err := p.Eval(`
import scriptling.mcp.tool as tool

no_request = tool.get_request() == None
empty_context = tool.request_context().get("user", "fallback")
`); err != nil {
		t.Fatalf("eval: %v", err)
	}

	if noReq, _ := p.GetVarAsBool("no_request"); !noReq {
		t.Fatal("get_request() should return None without an HTTP request")
	}
	if ctx, _ := p.GetVarAsString("empty_context"); ctx != "fallback" {
		t.Fatalf("request_context() default = %q, want fallback (dict should be empty)", ctx)
	}
}

// The runtime.jsonrpc accessors behave the same over stdio, where the method
// handler has no HTTP request behind it.
func TestJSONRPCRequestAccessorsOutsideHTTPRequest(t *testing.T) {
	p := scriptling.New()
	extlibs.RegisterRuntimeLibraryAll(p, nil)

	if _, err := p.Eval(`
import scriptling.runtime as runtime

no_request = runtime.jsonrpc.get_request() == None
empty_context = runtime.jsonrpc.request_context().get("user", "fallback")
`); err != nil {
		t.Fatalf("eval: %v", err)
	}

	if noReq, _ := p.GetVarAsBool("no_request"); !noReq {
		t.Fatal("get_request() should return None without an HTTP request")
	}
	if ctx, _ := p.GetVarAsString("empty_context"); ctx != "fallback" {
		t.Fatalf("request_context() default = %q, want fallback (dict should be empty)", ctx)
	}
}

// The accessors reject stray arguments instead of ignoring them.
func TestRequestAccessorsRejectArguments(t *testing.T) {
	for _, call := range []string{"tool.get_request(1)", "tool.request_context(1)"} {
		p := scriptling.New()
		extlibsmcp.RegisterToolHelpers(p)
		if _, err := p.Eval("import scriptling.mcp.tool as tool\n" + call); err == nil {
			t.Fatalf("%s should error with an argument", call)
		} else if !strings.Contains(err.Error(), "arguments") {
			t.Fatalf("%s error = %v, want an argument-count error", call, err)
		}
	}
}

// ── Unhappy middleware behaviour ─────────────────────────────────────────────

// A middleware that replaces request.context with something that is not a
// dict must not break handler-side access: request_context() falls back to an
// empty dict rather than handing handlers the wrong type.
func TestRequestContextNonDictFallsBackToEmptyDict(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
def check(request):
    request.context = "not a dict"
    return None
`))
	writeFile(t, libDir+"/apimod.py", []byte(`
import scriptling.mcp.tool as tool

def who(request):
    return {"body": tool.request_context().get("user", "fallback")}
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.route("/api/who", "apimod.who")
http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/who")
	if err != nil {
		t.Fatalf("GET /api/who: %v", err)
	}
	defer resp.Body.Close()
	body := make([]byte, 128)
	n, _ := resp.Body.Read(body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := strings.TrimSpace(string(body[:n])); got != "fallback" {
		t.Fatalf("handler saw %q, want fallback from empty dict", got)
	}
}

// A middleware that replaces request.context wholesale (rather than writing
// keys) is still picked up by the handler.
func TestMiddlewareReplacingContextWholesale(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
def check(request):
    request.context = {"user": "carol", "role": "wizard"}
    return None
`))
	writeFile(t, libDir+"/apimod.py", []byte(`
def who(request):
    return {"body": request.context["user"] + ":" + request.context["role"]}
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.route("/api/who", "apimod.who")
http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/who")
	if err != nil {
		t.Fatalf("GET /api/who: %v", err)
	}
	defer resp.Body.Close()
	body := make([]byte, 128)
	n, _ := resp.Body.Read(body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got := strings.TrimSpace(string(body[:n])); got != "carol:wizard" {
		t.Fatalf("handler saw %q, want carol:wizard", got)
	}
}

// ── JSON-RPC notifications and batches ───────────────────────────────────────

// writeJSONRPCContextServer builds a serving test server with token
// middleware that sets the context user, plus a method (who), a mutating
// method (mutate) and — when notePath is set — a notification (note) that
// records what it sees in a file.
func writeJSONRPCContextServer(t *testing.T, notePath string) *httptest.Server {
	t.Helper()
	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
def check(request):
    if request.header("authorization", "") != "Bearer bob-key":
        return {"status": 401, "body": "unauthorized"}
    request.context["user"] = "bob"
    return None
`))
	noteHandler := ""
	if notePath != "" {
		noteHandler = `

def note(params):
    import os
    user = runtime.jsonrpc.request_context().get("user", "anon")
    os.write_file("` + notePath + `", user)
`
	}
	writeFile(t, libDir+"/rpcmod.py", []byte(`
import scriptling.runtime as runtime

def who(params):
    return {
        "user": runtime.jsonrpc.request_context().get("user", "anon"),
        "extra": runtime.jsonrpc.request_context().get("extra", "none"),
    }

def mutate(params):
    ctx = runtime.jsonrpc.request_context()
    ctx["extra"] = "leaked"
    return ctx
`+noteHandler))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.middleware("authmod.check")
runtime.jsonrpc.method("who", "rpcmod.who")
runtime.jsonrpc.method("mutate", "rpcmod.mutate")
`+func() string {
		if notePath != "" {
			return "\nruntime.jsonrpc.notification(\"note\", \"rpcmod.note\")"
		}
		return ""
	}()+`
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{libDir}, JSONRPC: true})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts
}

// postJSONRPC posts a body to /json-rpc with the bearer token and returns the
// status plus the decoded response (nil for 204s).
func postJSONRPC(t *testing.T, ts *httptest.Server, body string) (int, any) {
	t.Helper()
	req, _ := http.NewRequest("POST", ts.URL+"/json-rpc", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer bob-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /json-rpc: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNoContent {
		return resp.StatusCode, nil
	}
	var parsed any
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp.StatusCode, parsed
}

func resultOf(t *testing.T, elem any) map[string]any {
	t.Helper()
	obj, ok := elem.(map[string]any)
	if !ok {
		t.Fatalf("batch element is %T, want object: %#v", elem, elem)
	}
	result, ok := obj["result"].(map[string]any)
	if !ok {
		t.Fatalf("no result object: %#v", elem)
	}
	return result
}

// waitForFile polls for a handler side effect written to path.
func waitForFile(t *testing.T, path string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if b, err := os.ReadFile(path); err == nil {
			return string(b)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
	return ""
}

// A notification has no id and produces no response, but its handler still
// runs with the middleware's context.
func TestMiddlewareContextReachesJSONRPCNotification(t *testing.T) {
	notePath := filepath.Join(t.TempDir(), "note.txt")
	ts := writeJSONRPCContextServer(t, notePath)

	if status, parsed := postJSONRPC(t, ts, `{"jsonrpc":"2.0","method":"note","params":{}}`); status != http.StatusNoContent || parsed != nil {
		t.Fatalf("notification = (%d, %#v), want (204, no body)", status, parsed)
	}
	if noted := waitForFile(t, notePath); noted != "bob" {
		t.Fatalf("notification handler saw %q, want bob", noted)
	}
}

// Every element of a batch sees the middleware's context.
func TestMiddlewareContextReachesJSONRPCBatch(t *testing.T) {
	ts := writeJSONRPCContextServer(t, "")

	status, parsed := postJSONRPC(t, ts, `[
		{"jsonrpc":"2.0","method":"who","params":{},"id":1},
		{"jsonrpc":"2.0","method":"who","params":{},"id":2}
	]`)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, parsed)
	}
	elems, ok := parsed.([]any)
	if !ok || len(elems) != 2 {
		t.Fatalf("batch response = %#v, want 2 elements", parsed)
	}
	for _, elem := range elems {
		if result := resultOf(t, elem); result["user"] != "bob" {
			t.Fatalf("batch element saw user=%v, want bob", result["user"])
		}
	}
}

// A mixed batch: the method responds, the notification runs, both share the
// middleware's context.
func TestMiddlewareContextJSONRPCMixedBatch(t *testing.T) {
	notePath := filepath.Join(t.TempDir(), "note.txt")
	ts := writeJSONRPCContextServer(t, notePath)

	status, parsed := postJSONRPC(t, ts, `[
		{"jsonrpc":"2.0","method":"who","params":{},"id":1},
		{"jsonrpc":"2.0","method":"note","params":{}}
	]`)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, parsed)
	}
	elems, ok := parsed.([]any)
	if !ok || len(elems) != 1 {
		t.Fatalf("mixed batch response = %#v, want 1 element (methods only)", parsed)
	}
	if result := resultOf(t, elems[0]); result["user"] != "bob" {
		t.Fatalf("method saw user=%v, want bob", result["user"])
	}
	if noted := waitForFile(t, notePath); noted != "bob" {
		t.Fatalf("notification in batch saw %q, want bob", noted)
	}
}

// Batch elements are dispatched concurrently, so request_context() hands each
// call its own copy: one handler's writes are invisible to the others in the
// same batch, and to later requests.
func TestJSONRPCContextMutationIsLocal(t *testing.T) {
	ts := writeJSONRPCContextServer(t, "")

	status, parsed := postJSONRPC(t, ts, `[
		{"jsonrpc":"2.0","method":"mutate","params":{},"id":1},
		{"jsonrpc":"2.0","method":"who","params":{},"id":2}
	]`)
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, parsed)
	}
	elems, ok := parsed.([]any)
	if !ok || len(elems) != 2 {
		t.Fatalf("batch response = %#v, want 2 elements", parsed)
	}

	byID := map[int]map[string]any{}
	for _, elem := range elems {
		obj, _ := elem.(map[string]any)
		id, _ := obj["id"].(float64)
		byID[int(id)] = resultOf(t, elem)
	}
	if got := byID[1]["extra"]; got != "leaked" {
		t.Fatalf("mutate saw extra=%v, want leaked (its own write)", got)
	}
	if got := byID[2]["extra"]; got != "none" {
		t.Fatalf("who saw extra=%v, want none (mutation must be local)", got)
	}

	// And the leak does not survive into the next request either.
	_, parsed = postJSONRPC(t, ts, `{"jsonrpc":"2.0","method":"who","params":{},"id":3}`)
	result := resultOf(t, parsed)
	if result["extra"] != "none" || result["user"] != "bob" {
		t.Fatalf("later request saw %#v, want extra=none user=bob", result)
	}
}

// ── WebSocket upgrades ───────────────────────────────────────────────────────

// The middleware guards WebSocket upgrades like every other endpoint, and the
// handler reads the context it populated from the upgrade request.
func TestWebSocketUpgradeGuardedByMiddleware(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/authmod.py", []byte(`
def check(request):
    if request.header("authorization", "") != "Bearer ws-key":
        return {"status": 401, "body": "unauthorized"}
    request.context["user"] = "wsalice"
    return None
`))
	writeFile(t, dir+"/wsgreet.py", []byte(`
import scriptling.mcp.tool as tool

def greet(client):
    user = tool.request_context().get("user", "anon")
    client.send("user:" + user)
    client.receive(timeout=5)
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.websocket("/ws", "wsgreet.greet")
http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	// Without the token the upgrade is rejected with the middleware's 401.
	if conn, resp, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Authorization": []string{"Bearer wrong"},
	}); err == nil {
		conn.Close()
		t.Fatal("dial with bad token should fail")
	} else if resp == nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("dial with bad token: status = %v, want 401", resp)
	}

	// With the token the handler runs and sees the middleware's context.
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, http.Header{
		"Authorization": []string{"Bearer ws-key"},
	})
	if err != nil {
		t.Fatalf("dial with token: %v", err)
	}
	defer conn.Close()

	if got := wsRead(t, conn); got != "user:wsalice" {
		t.Fatalf("ws handler saw %q, want user:wsalice", got)
	}
}

// A middleware that raises during a WebSocket upgrade surfaces as a 500 and
// the connection is not promoted.
func TestWebSocketUpgradeMiddlewareErrorReturns500(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir+"/authmod.py", []byte(`
def check(request):
    raise Exception("auth exploded")
`))
	writeFile(t, dir+"/wsecho.py", []byte(`
def echo(client):
    client.send("should not happen")
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.websocket("/ws", "wsecho.echo")
http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{ScriptFile: script, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	if conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil); err == nil {
		conn.Close()
		t.Fatal("dial should fail when the middleware raises")
	} else if resp == nil || resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("dial: status = %v, want 500", resp)
	}
}
