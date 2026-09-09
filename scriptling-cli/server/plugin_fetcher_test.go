package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/plugin"
)

// TestScriptFetcher pins runtime.plugin.register_fetcher: a script-declared
// fetcher reaches the plugin server, read answers string or bytes with None
// as a miss, and errors fail the call. This is how a script peer carries a
// host's declared assets inside itself.
func TestScriptFetcher(t *testing.T) {
	dir := t.TempDir()
	plugDir := filepath.Join(dir, "plug")
	if err := os.MkdirAll(plugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	modSrc := `def fetch_read(source, path):
    if path == "icon.svg":
        return "<svg>icon</svg>"
    if path == "blob.bin":
        return bytes([0, 1])
    return None
`
	if err := os.WriteFile(filepath.Join(plugDir, "mod.py"), []byte(modSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	setup := `import scriptling.runtime.plugin as plugin_srv
import scriptling.runtime as runtime

plugin_srv.serve("plug", "1.0")
plugin_srv.register_fetcher("plug", "plug.mod.fetch_read")
runtime.start_server()
`
	setupFile := writeSetup(t, setup)
	s, err := NewServer(ServerConfig{ScriptFile: setupFile, LibDirs: []string{dir}})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	s.buildPluginServer()
	if s.pluginServer == nil {
		t.Fatal("plugin server not built")
	}

	fetcher := scriptFetcher{s: s, read: extlibs.RuntimeState.PluginFetchRead}
	data, err := fetcher.Read(context.Background(), "plug://", "icon.svg")
	if err != nil || string(data) != "<svg>icon</svg>" {
		t.Fatalf("string read = %q, %v", data, err)
	}
	data, err = fetcher.Read(context.Background(), "plug://", "blob.bin")
	if err != nil || len(data) != 2 || data[0] != 0 {
		t.Fatalf("bytes read = %v, %v", data, err)
	}
	if _, err := fetcher.Read(context.Background(), "plug://", "missing.svg"); err == nil || err.Error() != plugin.ErrFetchNotFound.Error() {
		t.Fatalf("miss should be ErrFetchNotFound, got %v", err)
	}
	// Without a glob handler the fetcher answers no matches — a valid
	// answer per the fetcher contract.
	entries, err := fetcher.Glob(context.Background(), "plug://", "*")
	if err != nil || len(entries) != 0 {
		t.Fatalf("glob without handler = %v, %v", entries, err)
	}
}
