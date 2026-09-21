package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/paularlott/logger"
	mcplib "github.com/paularlott/mcp"
)

// TestBuildToolHandler_ReturnStructured_LegacyScript covers the imperative
// (non-decorated) script style: tool.return_structured() must set both
// StructuredContent and, per the MCP spec's backwards-compatibility
// guidance, a matching text content block.
func TestBuildToolHandler_ReturnStructured_LegacyScript(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, filepath.Join(dir, "report.py"), []byte(`
import scriptling.mcp.tool as tool
tool.return_structured({"records": [{"date": "2026-01-01", "amount": 42}]})
`))

	cfg := NewHandlerConfig(nil, WithLogger(logger.NewNullLogger()))
	handler, err := BuildToolHandler(script, cfg)
	if err != nil {
		t.Fatalf("BuildToolHandler: %v", err)
	}

	resp, err := handler(context.Background(), mcplib.NewToolRequest(nil))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	sc, ok := resp.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent type = %T, want map[string]any", resp.StructuredContent)
	}
	records, ok := sc["records"].([]any)
	if !ok || len(records) != 1 {
		t.Fatalf("StructuredContent = %+v", sc)
	}

	if len(resp.Content) != 1 || resp.Content[0].Type != "text" {
		t.Fatalf("expected a text fallback block, got %+v", resp.Content)
	}
	if resp.Content[0].Text == "" {
		t.Error("text fallback block is empty")
	}
}

// TestBuildToolHandler_ReturnStructured_RejectsNonDict is the unhappy path:
// structuredContent must be a JSON object per the MCP spec, so a list (or any
// other non-dict value) must be rejected with a clear error, not silently
// accepted as something a client could never validate against an outputSchema.
func TestBuildToolHandler_ReturnStructured_RejectsNonDict(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, filepath.Join(dir, "bad.py"), []byte(`
import scriptling.mcp.tool as tool
tool.return_structured([1, 2, 3])
`))

	cfg := NewHandlerConfig(nil, WithLogger(logger.NewNullLogger()))
	handler, err := BuildToolHandler(script, cfg)
	if err != nil {
		t.Fatalf("BuildToolHandler: %v", err)
	}

	if _, err := handler(context.Background(), mcplib.NewToolRequest(nil)); err == nil {
		t.Fatal("expected an error for a non-dict argument to return_structured")
	}
}

// TestBuildToolHandler_ReturnStructured_RequiresArgument covers calling
// return_structured() with no argument at all.
func TestBuildToolHandler_ReturnStructured_RequiresArgument(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, filepath.Join(dir, "empty.py"), []byte(`
import scriptling.mcp.tool as tool
tool.return_structured()
`))

	cfg := NewHandlerConfig(nil, WithLogger(logger.NewNullLogger()))
	handler, err := BuildToolHandler(script, cfg)
	if err != nil {
		t.Fatalf("BuildToolHandler: %v", err)
	}

	if _, err := handler(context.Background(), mcplib.NewToolRequest(nil)); err == nil {
		t.Fatal("expected an error for return_structured() with no argument")
	}
}

// TestBuildToolHandlerFunc_ReturnStructured covers the decorated-function
// style: the function body can still reach for tool.return_structured()
// directly for the structuredContent + fallback shape, instead of relying on
// the dict-return-value convention (which only produces text content).
func TestBuildToolHandlerFunc_ReturnStructured(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp
import scriptling.mcp.tool as tool

@mcp.tool("Report")
def report():
    tool.return_structured({"status": "ok"})
`)

	cfg := testHandlerConfig()
	handler := BuildToolHandlerFunc(src, "report", cfg)

	req := newToolRequest(t, map[string]interface{}{})
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	sc, ok := resp.StructuredContent.(map[string]any)
	if !ok || sc["status"] != "ok" {
		t.Fatalf("StructuredContent = %+v (%T)", resp.StructuredContent, resp.StructuredContent)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text == "" {
		t.Fatalf("expected a text fallback block, got %+v", resp.Content)
	}
}

// TestBuildToolHandler_ReturnStructured_PreservesLargeIntegers pins down that
// a large integer (beyond float64's 2^53 exact-integer range, e.g. a 64-bit
// database ID) survives structuredResponseFromJSON's JSON decode intact.
// Decoding into map[string]any without json.Decoder.UseNumber() converts
// every number to float64, which silently rounds an integer like this one —
// and since the result is re-marshaled for both the text fallback and the
// wire structuredContent, that rounding would otherwise reach the client.
func TestBuildToolHandler_ReturnStructured_PreservesLargeIntegers(t *testing.T) {
	dir := t.TempDir()
	const bigID = "123456789012345678" // > 2^53 (9007199254740992); exact in float64? no.
	script := writeScript(t, filepath.Join(dir, "report.py"), []byte(`
import scriptling.mcp.tool as tool
tool.return_structured({"id": `+bigID+`})
`))

	cfg := NewHandlerConfig(nil, WithLogger(logger.NewNullLogger()))
	handler, err := BuildToolHandler(script, cfg)
	if err != nil {
		t.Fatalf("BuildToolHandler: %v", err)
	}

	resp, err := handler(context.Background(), mcplib.NewToolRequest(nil))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}

	if !strings.Contains(resp.Content[0].Text, bigID) {
		t.Errorf("text fallback = %q, want it to contain the exact integer %s", resp.Content[0].Text, bigID)
	}

	sc, ok := resp.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("StructuredContent type = %T, want map[string]any", resp.StructuredContent)
	}
	id, ok := sc["id"].(json.Number)
	if !ok {
		t.Fatalf("id type = %T, want json.Number (got float64 means precision was already lost)", sc["id"])
	}
	if id.String() != bigID {
		t.Errorf("id = %s, want %s", id.String(), bigID)
	}
}
