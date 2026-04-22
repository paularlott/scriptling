package setup

import (
	"testing"

	logslog "github.com/paularlott/logger/slog"
	"github.com/paularlott/scriptling"
)

func TestScriptlingRegistersLoaderAndMCPLibrary(t *testing.T) {
	p := scriptling.New()
	log := logslog.New(logslog.Config{Level: "error"})

	Scriptling(p, []string{"/tmp/scriptling-lib"}, false, nil, nil, nil, log, "", "")

	if p.GetLibraryLoader() == nil {
		t.Fatal("expected library loader to be configured")
	}

	if err := p.Import("scriptling.mcp.tool"); err != nil {
		t.Fatalf("expected MCP tool helpers to be registered: %v", err)
	}
}
