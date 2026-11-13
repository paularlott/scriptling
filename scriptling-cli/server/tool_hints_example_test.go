package server

// Runs the examples/mcp-tool-hints example as a user would: its tools folder
// served over HTTP, and its own client.py pointed at the server. client.py
// asserts what it reads (each tool's title and hints, and which calls it
// would confirm first), so a change that drops a hint anywhere between the
// decorator or .toml sidecar and a script's client.tools() fails here.

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/logger"
	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/runtime"
	"github.com/paularlott/scriptling/extlibs/secretprovider"
	"github.com/paularlott/scriptling/scriptling-cli/setup"
)

func TestToolHintsExample(t *testing.T) {
	dir := filepath.Join("..", "..", "examples", "mcp-tool-hints")
	client, err := os.ReadFile(filepath.Join(dir, "client.py"))
	if err != nil {
		t.Fatalf("example client: %v", err)
	}

	setup.Factories(nil, nil, nil, secretprovider.NewRegistry(), logger.NewNullLogger(), "", "", false)
	runtime.ResetRuntime()
	s, err := NewServer(ServerConfig{MCPToolsDir: filepath.Join(dir, "tools")})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })
	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)

	p := scriptling.New()
	setup.Scriptling(p, nil, false, nil, nil, nil, logger.NewNullLogger(), "", "", false)
	setup.RegisterSys(p, []string{"client.py", ts.URL + "/mcp"})
	var out strings.Builder
	p.SetOutputWriter(&out)
	if _, err := p.Eval(string(client)); err != nil {
		t.Fatalf("client.py failed: %v\n%s", err, out.String())
	}
	for _, want := range []string{
		"notes__delete_note: ask the person first",
		"notes__legacy_export: ask the person first, reaches outside systems",
		"notes__search_notes -> 2 notes mention milk",
		"OK",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}
