package server

// Regression tests that drive the two shipped MCP Apps examples directly —
// the actual files under examples/mcp-app-dashboard and
// examples/mcp-prize-wheel, not synthesized inline scripts — so a future
// change to tool/resource scanning or serving that breaks either example is
// caught by `go test`, not just by the manual protocol/browser testing that
// verified them when they were built.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func serveMCPAppExample(t *testing.T, dir string) *httptest.Server {
	t.Helper()
	toolsDir := filepath.Join(dir, "tools")
	resourcesDir := filepath.Join(dir, "resources")
	if _, err := os.Stat(toolsDir); err != nil {
		t.Fatalf("example tools dir: %v", err)
	}
	if _, err := os.Stat(resourcesDir); err != nil {
		t.Fatalf("example resources dir: %v", err)
	}

	s, err := NewServer(ServerConfig{
		MCPToolsDir:     toolsDir,
		MCPResourcesDir: resourcesDir,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	t.Cleanup(func() { signalShutdown(t, s) })

	ts := httptest.NewServer(s.buildMux())
	t.Cleanup(ts.Close)
	return ts
}

func mcpCall(t *testing.T, ts *httptest.Server, method string, params string) map[string]any {
	t.Helper()
	body := `{"jsonrpc":"2.0","id":1,"method":"` + method + `"`
	if params != "" {
		body += `,"params":` + params
	}
	body += `}`
	status, parsed := mcpPost(t, ts, body, "")
	if status != http.StatusOK {
		t.Fatalf("%s: status %d, body %#v", method, status, parsed)
	}
	if errObj, ok := parsed["error"]; ok {
		t.Fatalf("%s: unexpected error %#v", method, errObj)
	}
	result, _ := parsed["result"].(map[string]any)
	if result == nil {
		t.Fatalf("%s: no result in %#v", method, parsed)
	}
	return result
}

func findToolByName(t *testing.T, tools []any, name string) map[string]any {
	t.Helper()
	for _, tl := range tools {
		if m, ok := tl.(map[string]any); ok && m["name"] == name {
			return m
		}
	}
	t.Fatalf("tool %q not found in %#v", name, tools)
	return nil
}

// --- examples/mcp-app-dashboard (the .toml-defined style) ---

var dashboardExampleDir = filepath.Join("..", "..", "examples", "mcp-app-dashboard")

func TestMCPAppDashboardExample(t *testing.T) {
	ts := serveMCPAppExample(t, dashboardExampleDir)

	tools, _ := mcpCall(t, ts, "tools/list", "")["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d: %#v", len(tools), tools)
	}

	report := findToolByName(t, tools, "sales_report")
	reportMeta, _ := report["_meta"].(map[string]any)
	reportUI, _ := reportMeta["ui"].(map[string]any)
	if reportUI["resourceUri"] != "ui://sales-dashboard/dashboard.html" {
		t.Errorf("sales_report resourceUri = %v", reportUI["resourceUri"])
	}
	reportIcons, _ := report["icons"].([]any)
	if len(reportIcons) != 1 {
		t.Errorf("sales_report icons = %v, want 1 entry", report["icons"])
	}

	addSale := findToolByName(t, tools, "add_sale")
	// Icons aren't required on every tool; add_sale intentionally has none.
	if _, hasIcons := addSale["icons"]; hasIcons {
		t.Errorf("add_sale should have no icons, got %v", addSale["icons"])
	}
	addSaleMeta, _ := addSale["_meta"].(map[string]any)
	addSaleUI, _ := addSaleMeta["ui"].(map[string]any)
	vis, _ := addSaleUI["visibility"].([]any)
	if len(vis) != 1 || vis[0] != "app" {
		t.Errorf("add_sale visibility = %v, want [app]", addSaleUI["visibility"])
	}
	// add_sale is only ever called by a dashboard that's already open, so it
	// has no view of its own to render — resourceUri is optional per spec
	// and must be omitted here.
	if _, hasResourceURI := addSaleUI["resourceUri"]; hasResourceURI {
		t.Errorf("add_sale should omit resourceUri (app-only action tool), got %v", addSaleUI["resourceUri"])
	}

	resResult := mcpCall(t, ts, "resources/read", `{"uri":"ui://sales-dashboard/dashboard.html"}`)
	contents, _ := resResult["contents"].([]any)
	content, _ := contents[0].(map[string]any)
	if content["mimeType"] != "text/html;profile=mcp-app" {
		t.Errorf("mimeType = %v", content["mimeType"])
	}
	html, _ := content["text"].(string)
	if len(html) < 100 {
		t.Errorf("dashboard HTML looks too short: %d bytes", len(html))
	}

	// _meta.ui on the wire, from the resource's own _dashboard.toml sidecar
	// ([ui] prefersBorder=true, [ui.csp] resourceDomains) — asserted at the
	// HTTP/JSON level, not just against the Go UIResourceMeta struct the
	// scanner produces internally, so a change to the JSON tags or the
	// server's serialization would be caught here too.
	contentMeta, _ := content["_meta"].(map[string]any)
	ui, ok := contentMeta["ui"].(map[string]any)
	if !ok {
		t.Fatalf("contents[0]._meta.ui missing or wrong type: %#v", content["_meta"])
	}
	if ui["prefersBorder"] != true {
		t.Errorf("_meta.ui.prefersBorder = %v, want true", ui["prefersBorder"])
	}
	csp, ok := ui["csp"].(map[string]any)
	if !ok {
		t.Fatalf("_meta.ui.csp missing or wrong type: %#v", ui)
	}
	if domains, _ := csp["resourceDomains"].([]any); len(domains) != 1 || domains[0] != "https://cdn.jsdelivr.net" {
		t.Errorf("_meta.ui.csp.resourceDomains = %v", csp["resourceDomains"])
	}

	report1 := mcpCall(t, ts, "tools/call", `{"name":"sales_report","arguments":{}}`)
	sc, _ := report1["structuredContent"].(map[string]any)
	records, _ := sc["records"].([]any)
	if len(records) != 4 {
		t.Fatalf("expected 4 seed records, got %d: %#v", len(records), sc)
	}

	addResult := mcpCall(t, ts, "tools/call", `{"name":"add_sale","arguments":{"date":"2026-09-15","product":"Widget","amount":9.99}}`)
	sc2, _ := addResult["structuredContent"].(map[string]any)
	records2, _ := sc2["records"].([]any)
	if len(records2) != 5 {
		t.Fatalf("expected 5 records after add_sale, got %d", len(records2))
	}
}

// --- examples/mcp-prize-wheel (the @mcp.tool decorator style, no .toml) ---

var wheelExampleDir = filepath.Join("..", "..", "examples", "mcp-prize-wheel")

func TestMCPPrizeWheelExample(t *testing.T) {
	ts := serveMCPAppExample(t, wheelExampleDir)

	tools, _ := mcpCall(t, ts, "tools/list", "")["tools"].([]any)
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d: %#v", len(tools), tools)
	}

	getHistory := findToolByName(t, tools, "get_history")
	getHistoryMeta, _ := getHistory["_meta"].(map[string]any)
	getHistoryUI, _ := getHistoryMeta["ui"].(map[string]any)
	// Same shape as claim_prize: app-only, no view of its own.
	if _, hasResourceURI := getHistoryUI["resourceUri"]; hasResourceURI {
		t.Errorf("get_history should omit resourceUri (app-only action tool), got %v", getHistoryUI["resourceUri"])
	}
	getHistoryVis, _ := getHistoryUI["visibility"].([]any)
	if len(getHistoryVis) != 1 || getHistoryVis[0] != "app" {
		t.Errorf("get_history visibility = %v, want [app]", getHistoryUI["visibility"])
	}

	spin := findToolByName(t, tools, "spin_wheel")
	spinMeta, _ := spin["_meta"].(map[string]any)
	spinUI, _ := spinMeta["ui"].(map[string]any)
	if spinUI["resourceUri"] != "ui://prize-wheel/wheel.html" {
		t.Errorf("spin_wheel resourceUri = %v", spinUI["resourceUri"])
	}
	spinIcons, _ := spin["icons"].([]any)
	if len(spinIcons) != 1 {
		t.Errorf("spin_wheel icons = %v, want 1 entry", spin["icons"])
	}

	claim := findToolByName(t, tools, "claim_prize")
	// Icons aren't required on every tool; claim_prize intentionally has none.
	if _, hasIcons := claim["icons"]; hasIcons {
		t.Errorf("claim_prize should have no icons, got %v", claim["icons"])
	}
	claimMeta, _ := claim["_meta"].(map[string]any)
	claimUI, _ := claimMeta["ui"].(map[string]any)
	// claim_prize is only ever called by a wheel view that's already open, so
	// it has no view of its own to render — resourceUri is optional per spec
	// and must be omitted here.
	if _, hasResourceURI := claimUI["resourceUri"]; hasResourceURI {
		t.Errorf("claim_prize should omit resourceUri (app-only action tool), got %v", claimUI["resourceUri"])
	}
	vis, _ := claimUI["visibility"].([]any)
	if len(vis) != 1 || vis[0] != "app" {
		t.Errorf("claim_prize visibility = %v, want [app]", claimUI["visibility"])
	}

	resResult := mcpCall(t, ts, "resources/read", `{"uri":"ui://prize-wheel/wheel.html"}`)
	contents, _ := resResult["contents"].([]any)
	content, _ := contents[0].(map[string]any)
	if content["mimeType"] != "text/html;profile=mcp-app" {
		t.Errorf("mimeType = %v", content["mimeType"])
	}

	spinResult := mcpCall(t, ts, "tools/call", `{"name":"spin_wheel","arguments":{}}`)
	sc, _ := spinResult["structuredContent"].(map[string]any)
	index, ok := sc["index"].(float64)
	if !ok {
		t.Fatalf("spin_wheel structuredContent = %#v, want an 'index'", sc)
	}
	prize, _ := sc["prize"].(string)
	if prize == "" {
		t.Fatalf("spin_wheel structuredContent = %#v, want a non-empty 'prize'", sc)
	}
	prizes, _ := sc["prizes"].([]any)
	if len(prizes) != 8 {
		t.Errorf("prizes = %v, want 8 entries", prizes)
	}
	// spin_wheel must also report the current claim history (initially
	// empty), not just claim_prize — this is what lets a freshly mounted (or
	// reloaded) wheel view show past winnings immediately, since the view
	// only ever receives whichever tool's result triggered its own mount.
	if h, _ := sc["history"].([]any); len(h) != 0 {
		t.Errorf("spin_wheel history (before any claim) = %v, want empty", h)
	}

	claimParams, _ := json.Marshal(map[string]any{"name": "claim_prize", "arguments": map[string]any{"index": index}})
	claimResult := mcpCall(t, ts, "tools/call", string(claimParams))
	sc2, _ := claimResult["structuredContent"].(map[string]any)
	history, _ := sc2["history"].([]any)
	if len(history) != 1 || history[0] != prize {
		t.Fatalf("claim_prize history = %v, want [%q]", history, prize)
	}

	// A later spin_wheel call must reflect the claim that just happened.
	spinResult2 := mcpCall(t, ts, "tools/call", `{"name":"spin_wheel","arguments":{}}`)
	sc3, _ := spinResult2["structuredContent"].(map[string]any)
	history2, _ := sc3["history"].([]any)
	if len(history2) != 1 || history2[0] != prize {
		t.Fatalf("spin_wheel history (after a claim) = %v, want [%q]", history2, prize)
	}

	// get_history must reflect a claim made after this spin's own mount —
	// the exact case its own snapshot can't cover (see wheel.html: it calls
	// get_history once on load specifically to recover from this).
	claimParams2, _ := json.Marshal(map[string]any{"name": "claim_prize", "arguments": map[string]any{"index": 0}})
	claimResult2 := mcpCall(t, ts, "tools/call", string(claimParams2))
	sc4, _ := claimResult2["structuredContent"].(map[string]any)
	claimedPrize2, _ := sc4["prize"].(string)

	getHistoryResult := mcpCall(t, ts, "tools/call", `{"name":"get_history","arguments":{}}`)
	sc5, _ := getHistoryResult["structuredContent"].(map[string]any)
	history3, _ := sc5["history"].([]any)
	if len(history3) != 2 || history3[0] != prize || history3[1] != claimedPrize2 {
		t.Fatalf("get_history = %v, want [%q, %q]", history3, prize, claimedPrize2)
	}

	// Unhappy path: an out-of-range index must error, not silently misbehave.
	status, body := mcpPost(t, ts, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"claim_prize","arguments":{"index":999}}}`, "")
	if status != http.StatusOK {
		t.Fatalf("claim_prize(999): unexpected HTTP status %d", status)
	}
	if _, ok := body["error"].(map[string]any); !ok {
		t.Fatalf("claim_prize(999) = %#v, want an error", body)
	}
}
