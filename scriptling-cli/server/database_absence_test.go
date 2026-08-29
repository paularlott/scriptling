//go:build !plugin_sqlite

package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The absence contract only holds without compiled-in drivers; with the
// plugin_sqlite tag the drivers register even with no plugin manager, which
// is exactly what the tagged compiled-in test proves.
// TestServerHandlerNoPluginsCleanAbsence proves a host with no plugin
// manager and no compiled-in drivers degrades cleanly: the setup import
// fails with a named unknown-library error at server start, not something
// mysterious at request time.
func TestServerHandlerNoPluginsCleanAbsence(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "setup.py"), []byte(dbSetupPy), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := NewServer(ServerConfig{
		ScriptFile: filepath.Join(dir, "setup.py"),
		LibDirs:    []string{dir},
	})
	if err == nil {
		t.Fatal("expected setup to fail without the sqlite library")
	}
	if !strings.Contains(err.Error(), "unknown library: scriptling.sqlite") {
		t.Fatalf("expected a named unknown-library error, got: %v", err)
	}
}
