package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mcplib "github.com/paularlott/mcp"
)

// A remote tool's execution error (isError: true) reaches the script as a
// raised error, never as an ordinary result; call_tools_parallel reports it
// in the "error" field.
func TestClientSurfacesToolExecutionErrors(t *testing.T) {
	server := mcplib.NewServer("err-server", "1.0")
	server.RegisterTool(mcplib.NewTool("fail", "Fails"),
		func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
			return mcplib.NewToolResponseError("quota exceeded"), nil
		})
	server.RegisterTool(mcplib.NewTool("ok", "Works"),
		func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
			return mcplib.NewToolResponseText("fine"), nil
		})
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	p := newMCPScriptling(t)
	result, err := p.Eval(`
import scriptling.mcp as mcp
c = mcp.Client("` + ts.URL + `")

assert c.call_tool("ok", {}) == "fine"

raised = ""
try:
    c.call_tool("fail", {})
except Exception as e:
    raised = str(e)
assert "MCP tool error: quota exceeded" in raised, "call_tool raised: " + raised

results = c.call_tools_parallel([
    {"name": "ok", "arguments": {}},
    {"name": "fail", "arguments": {}},
])
assert results[0].error == "" and results[0].result == "fine", "ok: " + str(results[0])
assert results[1].result == None, "fail result: " + str(results[1].result)
assert "quota exceeded" in results[1].error, "fail error: " + results[1].error

decoded = ""
try:
    mcp.decode_response({"isError": True, "content": [{"type": "text", "text": "bad"}]})
except Exception as e:
    decoded = str(e)
assert "MCP tool error: bad" in decoded, "decode_response raised: " + decoded

"OK"
`)
	if err != nil {
		t.Fatalf("script failed: %v", err)
	}
	if got, _ := result.AsString(); got != "OK" {
		t.Fatalf("got %q", got)
	}
}
