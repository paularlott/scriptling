package mcp_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A stdio server the script leaves running is shut down when the interpreter
// that started it is closed.
func TestClientStdioStoppedWithInterpreter(t *testing.T) {
	p := newMCPScriptling(t)
	stopped := filepath.Join(t.TempDir(), "stopped")
	if _, err := p.Eval(fmt.Sprintf(`
import scriptling.mcp as mcp
c = mcp.Client(%q, env=["SCRIPTLING_MCP_TEST_CHILD=1", "SCRIPTLING_MCP_TEST_STOPPED=%s"], namespace="child")
c.call_tool("child__env", {"name": "HOME"})
`, os.Args[0], stopped)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(stopped); err == nil {
		t.Fatal("the server stopped while the script still held its client")
	}
	p.Close()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(stopped); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the stdio server is still running after the interpreter closed")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
