package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildMiddlewareTestServer builds a Server whose setup script registers a
// script middleware (authmod.check — token auth via the Authorization header),
// one HTTP route, and a JSON-RPC method, plus a greet MCP tool. The returned
// httptest.Server serves the real buildMux() handler stack.
func buildMiddlewareTestServer(t *testing.T, setupExtra, middleware string) (*Server, *httptest.Server) {
	t.Helper()

	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
def check(request):
    token = request.header("authorization", "")
    if token == "Bearer alice-key" or token == "Bearer bob-key":
        return None
    return {"status": 401, "body": "unauthorized"}
`))
	writeFile(t, libDir+"/rpcmod.py", []byte("def echo(params):\n    return params\n"))
	writeFile(t, libDir+"/apimod.py", []byte("def who(request):\n    return {\"body\": \"ok\"}\n"))

	toolsDir := t.TempDir()
	writeFile(t, toolsDir+"/greet.toml", []byte("description = \"Greet a name\"\n\n[[parameters]]\nname=\"name\"\ntype=\"string\"\ndescription=\"Name to greet\"\nrequired=true\n"))
	writeFile(t, toolsDir+"/greet.py", []byte("import scriptling.mcp.tool as tool\ntool.return_string('hi ' + tool.get_string('name'))\n"))

	middlewareSetup := ""
	if middleware != "" {
		middlewareSetup = "\nhttp.middleware(\"" + middleware + "\")"
	}
	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.route("/api/who", "apimod.who")`+middlewareSetup+`
runtime.jsonrpc.method("echo", "rpcmod.echo")`+setupExtra+`
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{
		ScriptFile:  script,
		LibDirs:     []string{libDir},
		JSONRPC:     true,
		MCPToolsDir: toolsDir,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return s, ts
}

// mcpPost posts a single JSON-RPC request to the MCP endpoint.
func mcpPost(t *testing.T, ts *httptest.Server, body, authHeader string) (int, map[string]any) {
	t.Helper()
	req, err := http.NewRequest("POST", ts.URL+"/mcp", strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /mcp: %v", err)
	}
	defer resp.Body.Close()
	var parsed map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&parsed)
	return resp.StatusCode, parsed
}

func TestProtocolMiddlewareBlocksMCPWithoutToken(t *testing.T) {
	_, ts := buildMiddlewareTestServer(t, "", "authmod.check")

	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d: %#v", status, body)
	}
}

func TestProtocolMiddlewareBlocksMCPWithBadToken(t *testing.T) {
	_, ts := buildMiddlewareTestServer(t, "", "authmod.check")

	status, _ := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "Bearer wrong-key")
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 with bad token, got %d", status)
	}
}

func TestProtocolMiddlewareAllowsMCPWithToken(t *testing.T) {
	_, ts := buildMiddlewareTestServer(t, "", "authmod.check")

	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "Bearer alice-key")
	if status != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d: %#v", status, body)
	}
	result, _ := body["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	found := false
	for _, tl := range tools {
		if tool, ok := tl.(map[string]any); ok && tool["name"] == "greet" {
			found = true
		}
	}
	if !found {
		t.Fatalf("greet tool missing from tools/list: %#v", body)
	}
}

// The middleware consumes the request body to build its request object; the
// JSON-RPC body must still reach the MCP handler, or tool calls would fail.
func TestProtocolMiddlewareRestoresMCPBody(t *testing.T) {
	_, ts := buildMiddlewareTestServer(t, "", "authmod.check")

	status, body := mcpPost(t, ts,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"greet","arguments":{"name":"world"}}}`,
		"Bearer alice-key")
	if status != http.StatusOK {
		t.Fatalf("expected 200, got %d: %#v", status, body)
	}
	result, _ := body["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) < 1 {
		t.Fatalf("tool call produced no content: %#v", body)
	}
	first, _ := content[0].(map[string]any)
	if first["text"] != "hi world" {
		t.Fatalf("tool call result = %#v, want 'hi world'", first["text"])
	}
}

func TestProtocolMiddlewareBlocksMCGETStreamWithoutToken(t *testing.T) {
	_, ts := buildMiddlewareTestServer(t, "", "authmod.check")

	req, err := http.NewRequest("GET", ts.URL+"/mcp", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /mcp: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on GET /mcp without token, got %d", resp.StatusCode)
	}
}

func TestProtocolMiddlewareBlocksJSONRPCWithoutToken(t *testing.T) {
	_, ts := buildMiddlewareTestServer(t, "", "authmod.check")

	req, _ := http.NewRequest("POST", ts.URL+"/json-rpc", strings.NewReader(`{"jsonrpc":"2.0","method":"echo","params":{"hello":"world"},"id":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /json-rpc: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", resp.StatusCode)
	}
}

func TestProtocolMiddlewareAllowsJSONRPCWithToken(t *testing.T) {
	_, ts := buildMiddlewareTestServer(t, "", "authmod.check")

	req, _ := http.NewRequest("POST", ts.URL+"/json-rpc", strings.NewReader(`{"jsonrpc":"2.0","method":"echo","params":{"hello":"world"},"id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer bob-key")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /json-rpc: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with valid token, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result, _ := body["result"].(map[string]any)
	if result["hello"] != "world" {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestProtocolMiddlewareStillGuardsHTTPRoutes(t *testing.T) {
	_, ts := buildMiddlewareTestServer(t, "", "authmod.check")

	resp, err := http.Get(ts.URL + "/api/who")
	if err != nil {
		t.Fatalf("GET /api/who: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on route without token, got %d", resp.StatusCode)
	}

	req, _ := http.NewRequest("GET", ts.URL+"/api/who", nil)
	req.Header.Set("Authorization", "Bearer alice-key")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /api/who with token: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on route with token, got %d", resp2.StatusCode)
	}
}

// Without a registered middleware, protocol endpoints stay open (backwards
// compatibility) and a configured static bearer token guards everything.
func TestNoMiddlewareMCPOpenAccess(t *testing.T) {
	_, ts := buildMiddlewareTestServer(t, "", "")

	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "")
	if status != http.StatusOK {
		t.Fatalf("expected 200 without middleware, got %d: %#v", status, body)
	}
}

func TestNoMiddlewareJSONRPCOpenAccess(t *testing.T) {
	_, ts := buildMiddlewareTestServer(t, "", "")

	req, _ := http.NewRequest("POST", ts.URL+"/json-rpc", strings.NewReader(`{"jsonrpc":"2.0","method":"echo","params":{"a":1},"id":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /json-rpc: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 without middleware, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result, _ := body["result"].(map[string]any)
	if result["a"] != float64(1) {
		t.Fatalf("unexpected response: %#v", body)
	}
}

func TestStaticBearerTokenGuardsMCPIfNoMiddleware(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/rpcmod.py", []byte("def echo(params):\n    return params\n"))
	toolsDir := t.TempDir()
	writeFile(t, toolsDir+"/greet.toml", []byte("description = \"Greet\"\n"))
	writeFile(t, toolsDir+"/greet.py", []byte("import scriptling.mcp.tool as tool\ntool.return_string('hi')\n"))

	script := writeSetup(t, `
import scriptling.runtime as runtime
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	s, err := NewServer(ServerConfig{
		ScriptFile:  script,
		LibDirs:     []string{libDir},
		JSONRPC:     true,
		MCPToolsDir: toolsDir,
		BearerToken: "static-secret",
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	status, _ := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "")
	if status != http.StatusUnauthorized {
		t.Fatalf("expected 401 without static token, got %d", status)
	}
	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "Bearer static-secret")
	if status != http.StatusOK {
		t.Fatalf("expected 200 with static token, got %d: %#v", status, body)
	}

	// The static token guards /json-rpc too when no middleware is registered.
	req, _ := http.NewRequest("POST", ts.URL+"/json-rpc", strings.NewReader(`{"jsonrpc":"2.0","method":"echo","params":{"a":1},"id":1}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /json-rpc without token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on /json-rpc without static token, got %d", resp.StatusCode)
	}

	req, _ = http.NewRequest("POST", ts.URL+"/json-rpc", strings.NewReader(`{"jsonrpc":"2.0","method":"echo","params":{"a":1},"id":1}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer static-secret")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /json-rpc with static token: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on /json-rpc with static token, got %d", resp2.StatusCode)
	}
}

// Plugin-mode JSON-RPC (runtime.plugin.serve) mounts the plugin server at
// /json-rpc through the same middleware wrapper.
func TestProtocolMiddlewareGuardsPluginJSONRPC(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte(`
def check(request):
    token = request.header("authorization", "")
    if token == "Bearer alice-key":
        return None
    return {"status": 401, "body": "unauthorized"}
`))
	writeFile(t, libDir+"/plugmod.py", []byte("def add(a, b):\n    return a + b\n"))

	script := writeSetup(t, `
import scriptling.runtime.plugin as rp
import scriptling.runtime.http as http
import scriptling.runtime as runtime

rp.serve("plug", "1.0", "Plugin test")
rp.register_function("add", "plugmod.add")
http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)
	s, err := NewServer(ServerConfig{
		ScriptFile: script,
		LibDirs:    []string{libDir},
		JSONRPC:    true,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	handshake := `{"jsonrpc":"2.0","method":"scriptling.handshake","params":{},"id":1}`

	req, _ := http.NewRequest("POST", ts.URL+"/json-rpc", strings.NewReader(handshake))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /json-rpc without token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on plugin /json-rpc without token, got %d", resp.StatusCode)
	}

	req, _ = http.NewRequest("POST", ts.URL+"/json-rpc", strings.NewReader(handshake))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer alice-key")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /json-rpc with token: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on plugin /json-rpc with token, got %d", resp2.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&body); err != nil {
		t.Fatalf("decode handshake response: %v", err)
	}
	result, _ := body["result"].(map[string]any)
	lib, _ := result["library"].(map[string]any)
	if lib["name"] != "plug" {
		t.Fatalf("handshake library name mismatch: %#v", body)
	}
}

// A middleware that raises must surface as a 500, not silently pass through.
func TestProtocolMiddlewareErrorReturns500(t *testing.T) {
	libDir := t.TempDir()
	writeFile(t, libDir+"/authmod.py", []byte("def check(request):\n    raise Exception('auth exploded')\n"))
	toolsDir := t.TempDir()
	writeFile(t, toolsDir+"/greet.toml", []byte("description = \"Greet\"\n"))
	writeFile(t, toolsDir+"/greet.py", []byte("import scriptling.mcp.tool as tool\ntool.return_string('hi')\n"))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime
http.middleware("authmod.check")
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

	status, _ := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "Bearer alice-key")
	if status != http.StatusInternalServerError {
		t.Fatalf("expected 500 from failing middleware, got %d", status)
	}
}
