package mcp_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	scriptmcp "github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/extlibs/netsecurity"
)

// stdioScript starts this test binary as a stdio MCP server that writes marker
// when it starts, and reads back an env var through it.
func stdioScript(marker string) string {
	return fmt.Sprintf(`
import scriptling.mcp as mcp
c = mcp.Client(%q, env=["SCRIPTLING_MCP_TEST_CHILD=1", "SCRIPTLING_MCP_TEST_MARKER=%s", "PROBE=ok"], namespace="child")
r = c.call_tool("child__env", {"name": "PROBE"})
c.close()
r
`, os.Args[0], marker)
}

// A stdio MCP server is a subprocess: where the host disallows processes,
// mcp.Client() must refuse a command target before anything is started
// (CWE-78, vuln-0013), whatever the network policy says.
func TestClientStdioRefusedWhenProcessesDisallowed(t *testing.T) {
	for _, policy := range []*netsecurity.Config{nil, {}} {
		marker := filepath.Join(t.TempDir(), "started")
		p := scriptling.New()
		scriptmcp.RegisterConfigured(p, policy, false)
		_, err := p.Eval(stdioScript(marker))
		if err == nil || !strings.Contains(err.Error(), "subprocess") {
			t.Fatalf("policy=%v: expected a refusal naming subprocesses, got: %v", policy != nil, err)
		}
		if _, statErr := os.Stat(marker); statErr == nil {
			t.Fatalf("policy=%v: the stdio server was started despite being refused", policy != nil)
		}
		// Shell commands are refused the same way.
		if _, err := p.Eval(`import scriptling.mcp as mcp
mcp.Client("/bin/sh", args=["-c", "true"])`); err == nil || !strings.Contains(err.Error(), "subprocess") {
			t.Fatalf("policy=%v: /bin/sh was not refused: %v", policy != nil, err)
		}
	}
}

// HTTP clients are unaffected by the process restriction.
func TestClientHTTPAllowedWhenProcessesDisallowed(t *testing.T) {
	p := scriptling.New()
	scriptmcp.RegisterConfigured(p, nil, false)
	if _, err := p.Eval(`import scriptling.mcp as mcp
mcp.Client("https://example.com/mcp", namespace="h")`); err != nil {
		t.Fatalf("HTTP client should construct with processes disallowed: %v", err)
	}
}

// Where processes are allowed, stdio servers start and work as before, with
// or without a network policy (the policy only governs network access), and
// Register keeps its existing behaviour.
func TestClientStdioAllowedWhenProcessesAllowed(t *testing.T) {
	cases := map[string]func(*scriptling.Scriptling){
		"Register":                    func(p *scriptling.Scriptling) { scriptmcp.Register(p) },
		"Register with policy":        func(p *scriptling.Scriptling) { scriptmcp.Register(p, &netsecurity.Config{}) },
		"RegisterConfigured allowed":  func(p *scriptling.Scriptling) { scriptmcp.RegisterConfigured(p, nil, true) },
		"RegisterConfigured + policy": func(p *scriptling.Scriptling) { scriptmcp.RegisterConfigured(p, &netsecurity.Config{}, true) },
	}
	for name, register := range cases {
		t.Run(name, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "started")
			p := scriptling.New()
			register(p)
			result, err := p.Eval(stdioScript(marker))
			if err != nil {
				t.Fatalf("stdio client failed: %v", err)
			}
			if got := result.Inspect(); got != "ok" {
				t.Fatalf("call_tool returned %q, want ok", got)
			}
			if _, statErr := os.Stat(marker); statErr != nil {
				t.Fatal("the stdio server did not start")
			}
		})
	}
}
