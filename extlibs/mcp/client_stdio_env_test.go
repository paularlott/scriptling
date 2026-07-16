package mcp_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	mcplib "github.com/paularlott/mcp"
	"github.com/paularlott/scriptling/conversion"
)

// childModeEnv switches the test binary into "child MCP server" mode: instead
// of running tests it serves a tiny MCP stdio server exposing an "env" tool
// that reads its own environment. This lets the end-to-end test re-exec itself
// as a real subprocess and prove that env vars supplied to mcp.Client reach the
// child process.
const childModeEnv = "SCRIPTLING_MCP_TEST_CHILD"

func TestMain(m *testing.M) {
	if os.Getenv(childModeEnv) == "1" {
		runChildMCPServer()
		return
	}
	os.Exit(m.Run())
}

func runChildMCPServer() {
	s := mcplib.NewServer("scriptling-mcp-test", "0.0.0")
	s.RegisterTool(
		mcplib.NewTool("env", "Read an environment variable",
			mcplib.String("name", "the variable name", mcplib.Required()),
		),
		func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
			name, err := req.String("name")
			if err != nil {
				return nil, mcplib.NewToolErrorInvalidParams("name is required")
			}
			return mcplib.NewToolResponseText(os.Getenv(name)), nil
		},
	)
	_ = s.ServeStdio(context.Background())
}

// TestClientStdioEnvPassedThrough launches this test binary as a stdio MCP
// server via mcp.Client and verifies env vars reach the subprocess:
//   - an extra var supplied via env= is visible inside the child, and
//   - a var present in the parent environment is inherited (proving env=
//     merges on top of the inherited environment rather than replacing it).
func TestClientStdioEnvPassedThrough(t *testing.T) {
	// A variable present in the parent environment that the child must inherit.
	const parentProbe = "SCRIPTLING_TEST_INHERITED"
	t.Setenv(parentProbe, "from-parent")

	p := newMCPScriptling(t)

	// Re-exec this test binary as the stdio MCP server; the child-mode trigger
	// and the probe var are both carried in env=.
	bin := os.Args[0]
	result, err := p.Eval(fmt.Sprintf(`
import scriptling.mcp as mcp
c = mcp.Client(%q, env=["SCRIPTLING_MCP_TEST_CHILD=1", "SCRIPTLING_TEST_PROBE=hello-from-env"], namespace="child")
probe = c.call_tool("child__env", {"name": "SCRIPTLING_TEST_PROBE"})
inherited = c.call_tool("child__env", {"name": %q})
c.close()
[probe, inherited]
`, bin, parentProbe))
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}

	got, ok := conversion.ToGo(result).([]any)
	if !ok || len(got) != 2 {
		t.Fatalf("expected a 2-element list, got %T (%v)", result, result)
	}
	if got[0] != "hello-from-env" {
		t.Errorf("extra env var not passed through: got %v, want %q", got[0], "hello-from-env")
	}
	if got[1] != "from-parent" {
		t.Errorf("parent env var not inherited: got %v, want %q", got[1], "from-parent")
	}
}
