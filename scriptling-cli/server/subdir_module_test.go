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

func TestSplitHandlerRef(t *testing.T) {
	tests := []struct {
		ref     string
		wantMod string
		wantFn  string
		wantOK  bool
	}{
		{"handlers.get_user", "handlers", "get_user", true},
		{"routes.me.me", "routes.me", "me", true}, // dotted module in a subdirectory
		{"routes.me.get_user", "routes.me", "get_user", true},
		{"a.b.c.d", "a.b.c", "d", true},
		{"routes.me.Config", "routes.me", "Config", true},

		{"nolibrary", "", "", false},
		{".leading", "", "", false},
		{"trailing.", "", "", false},
	}

	for _, tt := range tests {
		mod, fn, ok := splitHandlerRef(tt.ref)
		if ok != tt.wantOK || mod != tt.wantMod || fn != tt.wantFn {
			t.Errorf("splitHandlerRef(%q) = (%q, %q, %v), want (%q, %q, %v)",
				tt.ref, mod, fn, ok, tt.wantMod, tt.wantFn, tt.wantOK)
		}
	}
}

// writeSubdirLib creates a temp lib dir with a handler module inside a
// subdirectory: routes/me.py (module name "routes.me").
func writeSubdirLib(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	routesDir := filepath.Join(dir, "routes")
	if err := os.Mkdir(routesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mePy := `import scriptling.runtime.http as http

@http.get("/me/{id}")
def me(request):
    return http.json(200, {"id": request.path_param("id")})
`
	if err := os.WriteFile(filepath.Join(routesDir, "me.py"), []byte(mePy), 0o644); err != nil {
		t.Fatal(err)
	}

	// A plain (non-decorated) module for the imperative registration form, so
	// importing it at request time does not trip the late-registration guard
	// for the decorated routes it does not share a module with.
	svcDir := filepath.Join(dir, "svc")
	if err := os.Mkdir(svcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plainPy := `import scriptling.runtime as runtime

def plain(request):
    return runtime.http.json(200, {"ok": True})
`
	if err := os.WriteFile(filepath.Join(svcDir, "plain.py"), []byte(plainPy), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestHTTPHandlerInSubdirectoryModule: a decorated handler in routes/me.py is
// registered as "routes.me.me"; the request-time re-import must import the
// dotted module "routes.me", not cut the ref at the first dot and fail with
// "unknown library: routes".
func TestHTTPHandlerInSubdirectoryModule(t *testing.T) {
	libDir := writeSubdirLib(t)
	setup := writeSetup(t, "import routes.me\n")

	s, err := NewServer(ServerConfig{
		ScriptFile: setup,
		LibDirs:    []string{libDir},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	if ref := s.handlers["GET /me/{id}"]; ref != "routes.me.me" {
		t.Fatalf("route ref = %q, want routes.me.me", ref)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	// Decorated handler in the subdirectory module.
	resp, err := http.Get(ts.URL + "/me/42")
	if err != nil {
		t.Fatalf("GET /me/42: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET /me/42 status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	var out map[string]any
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("invalid JSON %q: %v", body, err)
	}
	if out["id"] != "42" {
		t.Errorf("id = %v, want \"42\"", out["id"])
	}
}

// TestHTTPImperativeRefInSubdirectoryModule: the imperative registration form
// with a dotted ref ("svc.plain.plain") dispatches the same way.
func TestHTTPImperativeRefInSubdirectoryModule(t *testing.T) {
	libDir := writeSubdirLib(t)
	setup := writeSetup(t, `
import scriptling.runtime as runtime
runtime.http.get("/plain", "svc.plain.plain")
`)

	s, err := NewServer(ServerConfig{
		ScriptFile: setup,
		LibDirs:    []string{libDir},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/plain")
	if err != nil {
		t.Fatalf("GET /plain: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("GET /plain status = %d, want 200 (body %s)", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), `"ok":true`) {
		t.Errorf("body = %s, want ok:true", body)
	}
}

// TestJSONRPCHandlerInSubdirectoryModule: JSON-RPC methods registered from a
// module inside a subdirectory re-import correctly at request time.
func TestJSONRPCHandlerInSubdirectoryModule(t *testing.T) {
	dir := t.TempDir()
	rpcDir := filepath.Join(dir, "rpcmod")
	if err := os.Mkdir(rpcDir, 0o755); err != nil {
		t.Fatal(err)
	}
	subPy := `def echo(params):
    return params
`
	if err := os.WriteFile(filepath.Join(rpcDir, "sub.py"), []byte(subPy), 0o644); err != nil {
		t.Fatal(err)
	}

	setup := writeSetup(t, `
import scriptling.runtime as runtime
runtime.jsonrpc.method("echo", "rpcmod.sub.echo")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{
		ScriptFile: setup,
		LibDirs:    []string{dir},
		JSONRPC:    true,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	// The setup script loops on server_running(): release it so its goroutine
	// does not leak into the next test's NewServer (whose ResetRuntime window
	// it could then crash — "close of nil channel").
	defer signalShutdown(t, s)

	httpSrv := httptest.NewServer(http.HandlerFunc(s.handleJSONRPCHTTP))
	defer httpSrv.Close()

	resp, err := http.Post(httpSrv.URL, "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","method":"echo","params":{"hi":"there"},"id":1}`))
	if err != nil {
		t.Fatalf("POST /json-rpc: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	result, ok := body["result"].(map[string]any)
	if !ok || result["hi"] != "there" {
		t.Fatalf("unexpected response: %#v", body)
	}
}
