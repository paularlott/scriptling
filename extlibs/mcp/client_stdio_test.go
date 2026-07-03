package mcp_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	scriptmcp "github.com/paularlott/scriptling/extlibs/mcp"
)

// newMCPScriptling returns a scriptling instance with the mcp library
// registered as "scriptling.mcp".
func newMCPScriptling(t *testing.T) *scriptling.Scriptling {
	t.Helper()
	p := scriptling.New()
	scriptmcp.Register(p)
	return p
}

func TestClientHTTPConstructs(t *testing.T) {
	p := newMCPScriptling(t)
	// Constructing an HTTP client must not error (no connection is made yet).
	_, err := p.Eval(`import scriptling.mcp as mcp
c = mcp.Client("https://example.com/mcp", namespace="t2", bearer_token="secret")
c
`)
	if err != nil {
		t.Fatalf("http client construct failed: %v", err)
	}
}

func TestClientURLWithArgsRejected(t *testing.T) {
	p := newMCPScriptling(t)
	_, err := p.Eval(`import scriptling.mcp as mcp
mcp.Client("https://example.com/mcp", args=["--x"])
`)
	if err == nil || !strings.Contains(err.Error(), "args") {
		t.Fatalf("expected an 'args' rejection error, got: %v", err)
	}
}

func TestClientCommandWithBearerRejected(t *testing.T) {
	p := newMCPScriptling(t)
	_, err := p.Eval(`import scriptling.mcp as mcp
mcp.Client("/path/to/binary", bearer_token="secret")
`)
	if err == nil || !strings.Contains(err.Error(), "bearer_token") {
		t.Fatalf("expected a 'bearer_token' rejection error, got: %v", err)
	}
}

func TestClientStdioBadCommand(t *testing.T) {
	p := newMCPScriptling(t)
	// A non-existent command should surface a start error from mcp.Client.
	_, err := p.Eval(`import scriptling.mcp as mcp
mcp.Client("/nonexistent/scriptling-mcp-binary-xyz", args=["--mcp-exec-script"])
`)
	if err == nil {
		t.Fatal("expected an error launching a non-existent stdio server")
	}
}
