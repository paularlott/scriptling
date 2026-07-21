package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestDecoratorHTTPIntegration proves the full chain:
// setup.py imports a library file with @http.get decorators →
// NewServer collects the registered routes →
// buildMux serves them →
// runHandler imports the library and calls the function on a fresh evaluator.
func TestDecoratorHTTPIntegration(t *testing.T) {
	dir := t.TempDir()

	// Handler library with decorators.
	handlersPy := `import scriptling.runtime.http as http

@http.get("/health")
def health(request):
    return http.json(200, {"status": "ok"})

@http.post("/echo")
def echo(request):
    return http.json(200, request.json())

@http.route("/items", methods=["GET", "POST"])
def items(request):
    return http.json(200, {"method": request.method})

@http.middleware
def auth(request):
    if request.path == "/items" and request.method == "POST":
        return http.json(403, {"error": "forbidden"})
    return None

@http.get("/checked")
def checked(request):
    return http.json(200, {"ok": True})

@http.not_found
def handle_404(request):
    return http.json(404, {"error": "custom 404"})
`
	if err := os.WriteFile(filepath.Join(dir, "handlers.py"), []byte(handlersPy), 0o644); err != nil {
		t.Fatal(err)
	}

	// Setup script just imports the handlers library.
	setupPy := `import handlers
`
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte(setupPy), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(ServerConfig{
		ScriptFile: filepath.Join(dir, "setup.py"),
		LibDirs:    []string{dir},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	// GET /health — decorated GET handler
	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /health: status %d, want 200", resp.StatusCode)
	}
	var result map[string]any
	json.Unmarshal(body, &result)
	if result["status"] != "ok" {
		t.Errorf("GET /health: body %s, want {\"status\":\"ok\"}", body)
	}

	// POST /echo — decorated POST handler (middleware allows)
	resp, err = http.Post(ts.URL+"/echo", "application/json",
		strings.NewReader(`{"msg":"hi"}`))
	if err != nil {
		t.Fatalf("POST /echo: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("POST /echo: status %d, want 200", resp.StatusCode)
	}

	// GET /items — route with methods=["GET","POST"]
	resp, err = http.Get(ts.URL + "/items")
	if err != nil {
		t.Fatalf("GET /items: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /items: status %d", resp.StatusCode)
	}
	json.Unmarshal(body, &result)
	if result["method"] != "GET" {
		t.Errorf("GET /items: body %s", body)
	}

	// POST /items — middleware blocks POST on /items
	resp, err = http.Post(ts.URL+"/items", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /items: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != 403 {
		t.Errorf("POST /items: status %d, want 403 (middleware)", resp.StatusCode)
	}

	// GET /checked — middleware allows GET
	resp, err = http.Get(ts.URL + "/checked")
	if err != nil {
		t.Fatalf("GET /checked: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /checked: status %d, want 200", resp.StatusCode)
	}

	// GET /nonexistent — not_found handler returns custom JSON 404
	resp, err = http.Get(ts.URL + "/nonexistent")
	if err != nil {
		t.Fatalf("GET /nonexistent: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("GET /nonexistent: status %d, want 404", resp.StatusCode)
	}
	json.Unmarshal(body, &result)
	if result["error"] != "custom 404" {
		t.Errorf("GET /nonexistent: body %s, want custom 404", body)
	}
}

// TestDecoratorInSetupScript proves decorators work when used directly in
// the setup/main script (the __file__ fallback path for module name resolution).
func TestDecoratorInSetupScript(t *testing.T) {
	dir := t.TempDir()

	// Single-file app: setup.py has decorators directly.
	setupPy := `import scriptling.runtime.http as http

@http.get("/ping")
def ping(request):
    return http.json(200, {"pong": True})
`
	if err := os.WriteFile(filepath.Join(dir, "app.py"), []byte(setupPy), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(ServerConfig{
		ScriptFile: filepath.Join(dir, "app.py"),
		LibDirs:    []string{dir},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	// The decorator resolved module name from __file__ ("app.py" → "app").
	// runHandler must be able to import "app" and find ping().
	resp, err := http.Get(ts.URL + "/ping")
	if err != nil {
		t.Fatalf("GET /ping: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET /ping: status %d, want 200", resp.StatusCode)
	}
	var result map[string]any
	json.Unmarshal(body, &result)
	if result["pong"] != true {
		t.Errorf("GET /ping: body %s, want {\"pong\":true}", body)
	}
}

// TestDecoratorJSONRPCIntegration proves @jsonrpc.method decorators register
// methods that the JSON-RPC server can dispatch.
func TestDecoratorJSONRPCIntegration(t *testing.T) {
	dir := t.TempDir()

	handlersPy := `import scriptling.runtime.jsonrpc as jsonrpc

@jsonrpc.method("echo")
def echo(params):
    return params

@jsonrpc.method("double")
def double(params):
    return params["n"] * 2
`
	if err := os.WriteFile(filepath.Join(dir, "handlers.py"), []byte(handlersPy), 0o644); err != nil {
		t.Fatal(err)
	}

	setupPy := `import handlers
`
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte(setupPy), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := NewServer(ServerConfig{
		ScriptFile: filepath.Join(dir, "setup.py"),
		LibDirs:    []string{dir},
		JSONRPC:    true,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(s.handleJSONRPCHTTP))
	defer ts.Close()

	// echo
	resp, err := http.Post(ts.URL, "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","method":"echo","params":{"hello":"world"},"id":1}`))
	if err != nil {
		t.Fatalf("echo: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var resp1 map[string]any
	json.Unmarshal(body, &resp1)
	if resp1["result"] == nil {
		t.Errorf("echo: response %s, missing result", body)
	}

	// double
	resp, err = http.Post(ts.URL, "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","method":"double","params":{"n":21},"id":2}`))
	if err != nil {
		t.Fatalf("double: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	resp.Body.Close()
	var resp2 map[string]any
	json.Unmarshal(body, &resp2)
	if result, ok := resp2["result"].(float64); !ok || result != 42 {
		t.Errorf("double: response %s, want result 42", body)
	}
}
