package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// TestHealthBuiltIn pins the default: GET /health answers 200 "OK", and HEAD
// rides the same registration (ServeMux dispatches HEAD to GET patterns).
func TestHealthBuiltIn(t *testing.T) {
	s, err := NewServer(ServerConfig{})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "OK" {
		t.Fatalf("built-in health: %d %q", resp.StatusCode, body)
	}

	head, err := http.NewRequest(http.MethodHead, ts.URL+"/health", nil)
	if err != nil {
		t.Fatal(err)
	}
	hresp, err := http.DefaultClient.Do(head)
	if err != nil {
		t.Fatalf("HEAD /health: %v", err)
	}
	hresp.Body.Close()
	if hresp.StatusCode != 200 {
		t.Fatalf("HEAD /health: %d", hresp.StatusCode)
	}
}

// TestHealthScriptRouteClaim pins the option for a custom health handler: a
// setup script registering GET /health replaces the built-in responder —
// the claim mechanism is the documented surface, so a regression here breaks
// every deployment that relies on it.
func TestHealthScriptRouteClaim(t *testing.T) {
	setup := writeSetup(t, `import scriptling.runtime.http as http

@http.get("/health")
def health(request):
    return {"status": 200, "body": "script-health"}
`)
	s, err := NewServer(ServerConfig{
		ScriptFile: setup,
		LibDirs:    []string{filepath.Dir(setup)},
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.buildMux())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 || string(body) != "script-health" {
		t.Fatalf("script health: %d %q", resp.StatusCode, body)
	}
}
