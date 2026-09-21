package server

// Test for register_request_tool's "ui" kwarg (MCP Apps SEP-1865 linkage on a
// per-request/dynamic tool), a separate concern from the visibility/collision
// tests in request_scoped_mcp_test.go.

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func writeRequestUIToolServer(t *testing.T) *httptest.Server {
	t.Helper()
	libDir := t.TempDir()

	writeFile(t, libDir+"/authmod.py", []byte(`
import scriptling.runtime.mcp as mcp

def check(request):
    request.context["user"] = "alice"
    mcp.register_request_tool("sales_report", handler="reportmod.report",
        description="Get the sales report",
        ui={"resourceUri": "ui://sales-dashboard/dashboard.html", "visibility": ["model", "app"]})
    return None
`))
	writeFile(t, libDir+"/reportmod.py", []byte(`
def report():
    return {"records": []}
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
		MCPExecTool: true, // enables MCP mode; the tool under test is registered dynamically by middleware
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts
}

func TestRequestTool_UILinksResource(t *testing.T) {
	ts := writeRequestUIToolServer(t)

	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "")
	if status != http.StatusOK {
		t.Fatalf("tools/list: %d %#v", status, body)
	}
	result, _ := body["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	var tool map[string]any
	for _, tl := range tools {
		if m, ok := tl.(map[string]any); ok && m["name"] == "sales_report" {
			tool = m
			break
		}
	}
	if tool == nil {
		t.Fatalf("sales_report not found in tools/list: %#v", tools)
	}
	meta, ok := tool["_meta"].(map[string]any)
	if !ok {
		t.Fatalf("missing _meta: %#v", tool)
	}
	ui, ok := meta["ui"].(map[string]any)
	if !ok {
		t.Fatalf("missing _meta.ui: %#v", meta)
	}
	if ui["resourceUri"] != "ui://sales-dashboard/dashboard.html" {
		t.Errorf("resourceUri = %v", ui["resourceUri"])
	}
	vis, _ := ui["visibility"].([]any)
	if len(vis) != 2 || vis[0] != "model" || vis[1] != "app" {
		t.Errorf("visibility = %v", ui["visibility"])
	}
}

func writeRequestIconsToolServer(t *testing.T) *httptest.Server {
	t.Helper()
	libDir := t.TempDir()

	writeFile(t, libDir+"/authmod.py", []byte(`
import scriptling.runtime.mcp as mcp

def check(request):
    request.context["user"] = "alice"
    mcp.register_request_tool("weather", handler="weathermod.forecast",
        description="Get the weather",
        icons=[{"src": "https://example.com/weather.png", "mimeType": "image/png"}])
    return None
`))
	writeFile(t, libDir+"/weathermod.py", []byte(`
def forecast():
    return {"forecast": "sunny"}
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
		MCPExecTool: true,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts
}

func TestRequestTool_Icons(t *testing.T) {
	ts := writeRequestIconsToolServer(t)

	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`, "")
	if status != http.StatusOK {
		t.Fatalf("tools/list: %d %#v", status, body)
	}
	result, _ := body["result"].(map[string]any)
	tools, _ := result["tools"].([]any)
	var tool map[string]any
	for _, tl := range tools {
		if m, ok := tl.(map[string]any); ok && m["name"] == "weather" {
			tool = m
			break
		}
	}
	if tool == nil {
		t.Fatalf("weather not found in tools/list: %#v", tools)
	}
	icons, _ := tool["icons"].([]any)
	if len(icons) != 1 {
		t.Fatalf("expected 1 icon, got %#v", tool["icons"])
	}
	icon, _ := icons[0].(map[string]any)
	if icon["src"] != "https://example.com/weather.png" || icon["mimeType"] != "image/png" {
		t.Errorf("icon = %#v", icon)
	}
}
