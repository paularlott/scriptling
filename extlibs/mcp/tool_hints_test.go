package mcp_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	mcplib "github.com/paularlott/mcp"
)

// tools() gives each tool's title and behaviour hints, named as mcp.tool
// declares them, with None for a hint the server did not state, so a script
// can tell read-only tools from destructive ones before calling them. With and
// without a namespace: the mcp library once dropped both when it rebuilt the
// tool list.
func TestClientToolsCarryTitleAndHints(t *testing.T) {
	ok := func(ctx context.Context, req *mcplib.ToolRequest) (*mcplib.ToolResponse, error) {
		return mcplib.NewToolResponseText("ok"), nil
	}
	server := mcplib.NewServer("hints-server", "1.0")
	server.RegisterTool(mcplib.NewTool("get_note", "Read a note").Title("Get Note").ReadOnly().OpenWorld(false), ok)
	server.RegisterTool(mcplib.NewTool("delete_note", "Delete a note").Title("Delete Note").Destructive(true).Idempotent(), ok)
	server.RegisterTool(mcplib.NewTool("plain", "No hints"), ok)
	ts := httptest.NewServer(http.HandlerFunc(server.HandleRequest))
	defer ts.Close()

	for _, ns := range []string{"", "notes"} {
		t.Run("namespace="+ns, func(t *testing.T) {
			prefix := ""
			if ns != "" {
				prefix = ns + "__"
			}
			p := newMCPScriptling(t)
			result, err := p.Eval(`
import scriptling.mcp as mcp
c = mcp.Client("` + ts.URL + `", namespace="` + ns + `")
tools = {}
for t in c.tools():
    tools[t.name] = t

g = tools["` + prefix + `get_note"]
assert g.title == "Get Note", "title: " + str(g.title)
assert g.read_only == True and g.open_world == False, str(g)
assert g.destructive == None and g.idempotent == None, str(g)

d = tools["` + prefix + `delete_note"]
assert d.title == "Delete Note", "title: " + str(d.title)
assert d.destructive == True and d.idempotent == True, str(d)
assert d.read_only == None and d.open_world == None, str(d)
assert d.destructive != False and not d.read_only

n = tools["` + prefix + `plain"]
assert n.title == "" and n.read_only == None and n.destructive == None, str(n)
"OK"
`)
			if err != nil {
				t.Fatalf("script failed: %v", err)
			}
			if got, _ := result.AsString(); got != "OK" {
				t.Fatalf("got %q", got)
			}
		})
	}
}
