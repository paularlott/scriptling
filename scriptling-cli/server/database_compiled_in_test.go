//go:build plugin_sqlite

package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestServerHandlerCompiledIn proves a default-scriptling-style host — drivers
// compiled in, no plugin manager at all — serves database handlers: the
// compiled-in registration happens inside RegisterLibraries, which the
// server calls on every request environment regardless of manager.
func TestServerHandlerCompiledIn(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte(dbSetupPy), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := NewServer(ServerConfig{
		ScriptFile: filepath.Join(dir, "setup.py"),
		LibDirs:    []string{dir},
		// no PluginManager: the drivers are compiled in
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/db?name=compiled-in")
	if err != nil {
		t.Fatalf("GET /db: %v", err)
	}
	defer resp.Body.Close()
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload["name"] != "compiled-in" {
		t.Fatalf("handler result: %v", payload)
	}
}
