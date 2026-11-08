package server

// Tests that a tool's title and behaviour hints (read_only, destructive,
// idempotent, open_world) reach tools/list as the spec's title and
// annotations, for each way scriptling defines an MCP tool: a .toml + .py
// pair, an @mcp.tool decorator, and register_request_tool from middleware.

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func writeToolAnnotationsServer(t *testing.T) *httptest.Server {
	t.Helper()
	toolsDir := t.TempDir()
	libDir := t.TempDir()

	// .toml-defined tools: one with every option, one with none.
	writeFile(t, toolsDir+"/delete_note.toml", []byte(`description = "Delete a note"
title = "Delete Note"
destructive = true
idempotent = true
open_world = false

[[parameters]]
name = "id"
type = "string"
description = "Note ID"
required = true
`))
	writeFile(t, toolsDir+"/delete_note.py", []byte("import scriptling.mcp.tool as tool\ntool.return_string('deleted')\n"))
	writeFile(t, toolsDir+"/plain_toml.toml", []byte("description = \"No hints\"\n"))
	writeFile(t, toolsDir+"/plain_toml.py", []byte("import scriptling.mcp.tool as tool\ntool.return_string('ok')\n"))

	// Decorated tools: one with a title and a hint, one with none (None
	// leaves a hint unset just like leaving it out).
	writeFile(t, toolsDir+"/notes.py", []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Search notes", title="Search Notes", read_only=True, open_world=False, destructive=None)
def search_notes(query):
    return "found " + query

@mcp.tool("No hints")
def plain_decorated():
    return "ok"
`))

	// A request-scoped tool registered by middleware.
	writeFile(t, libDir+"/authmod.py", []byte(`
import scriptling.runtime.mcp as mcp

def check(request):
    mcp.register_request_tool("restart_service", handler="adminmod.restart",
        description="Restart a service", title="Restart Service",
        destructive=False, idempotent=True, read_only=False)
    return None
`))
	writeFile(t, libDir+"/adminmod.py", []byte(`
def restart():
    return "restarted"
`))

	script := writeSetup(t, `
import scriptling.runtime.http as http
import scriptling.runtime as runtime

http.middleware("authmod.check")
runtime.start_server(wait=False)
while runtime.server_running():
    yield_now()
`)

	s, err := NewServer(ServerConfig{
		ScriptFile:  script,
		LibDirs:     []string{libDir},
		MCPToolsDir: toolsDir,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts
}

func TestToolTitleAndAnnotationsInToolsList(t *testing.T) {
	ts := writeToolAnnotationsServer(t)

	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "")
	if status != http.StatusOK {
		t.Fatalf("tools/list: %d %#v", status, body)
	}
	result, _ := body["result"].(map[string]any)
	tools, _ := result["tools"].([]any)

	cases := []struct {
		tool        string
		title       any // nil = no title key
		annotations map[string]any
	}{
		{"delete_note", "Delete Note", map[string]any{
			"destructiveHint": true, "idempotentHint": true, "openWorldHint": false,
		}},
		{"search_notes", "Search Notes", map[string]any{
			"readOnlyHint": true, "openWorldHint": false,
		}},
		{"restart_service", "Restart Service", map[string]any{
			"readOnlyHint": false, "destructiveHint": false, "idempotentHint": true,
		}},
		{"plain_toml", nil, nil},
		{"plain_decorated", nil, nil},
	}
	for _, c := range cases {
		t.Run(c.tool, func(t *testing.T) {
			tool := findToolByName(t, tools, c.tool)
			if got, ok := tool["title"]; c.title == nil && ok {
				t.Errorf("title = %v, want none", got)
			} else if c.title != nil && got != c.title {
				t.Errorf("title = %v, want %v", got, c.title)
			}
			ann, ok := tool["annotations"].(map[string]any)
			if c.annotations == nil {
				if _, present := tool["annotations"]; present {
					t.Errorf("annotations = %#v, want none", tool["annotations"])
				}
				return
			}
			if !ok {
				t.Fatalf("missing annotations: %#v", tool)
			}
			if !reflect.DeepEqual(ann, c.annotations) {
				t.Errorf("annotations = %#v, want %#v", ann, c.annotations)
			}
		})
	}
}
