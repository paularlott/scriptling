//go:build !plugin_sqlite

package mcp

import (
	"strings"
	"testing"
)

// The absence contract only holds without compiled-in drivers; with the
// plugin_sqlite tag the drivers register even with no plugin manager.
// TestPrepareScriptlingCleanAbsence proves a host with no plugins and no
// compiled-in drivers degrades to a named unknown-library error.
func TestPrepareScriptlingCleanAbsence(t *testing.T) {
	p := prepareScriptling(NewHandlerConfig(nil), nil)
	if p == nil {
		t.Fatal("prepareScriptling returned nil")
	}
	_, err := p.Eval("import scriptling.sqlite as sqlite")
	if err == nil {
		t.Fatal("expected import to fail without the sqlite library")
	}
	if !strings.Contains(err.Error(), "unknown library") {
		t.Fatalf("expected unknown-library error, got: %v", err)
	}
}
