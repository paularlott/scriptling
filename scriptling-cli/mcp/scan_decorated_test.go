package mcp

import (
	"testing"
	"testing/fstest"
)

func TestScanDecoratedToolsSingle(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Calculate an expression", params={"expr": "Math expression to evaluate"})
def calc(expr):
    return str(eval(expr))
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	tool := tools[0]
	if tool.Name != "calc" {
		t.Errorf("name: expected 'calc', got %q", tool.Name)
	}
	if tool.FuncName != "calc" {
		t.Errorf("funcName: expected 'calc', got %q", tool.FuncName)
	}
	if tool.Meta.Description != "Calculate an expression" {
		t.Errorf("description: got %q", tool.Meta.Description)
	}
	if len(tool.Meta.Parameters) != 1 {
		t.Fatalf("expected 1 param, got %d", len(tool.Meta.Parameters))
	}

	p := tool.Meta.Parameters[0]
	if p.Name != "expr" {
		t.Errorf("param name: expected 'expr', got %q", p.Name)
	}
	if p.Type != "string" {
		t.Errorf("param type: expected 'string', got %q", p.Type)
	}
	if p.Description != "Math expression to evaluate" {
		t.Errorf("param description: got %q", p.Description)
	}
	if !p.Required {
		t.Error("param should be required (no default)")
	}
}

func TestScanDecoratedToolsMultiple(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Encode text to base64", params={"text": "Text to encode"})
def encode_base64(text):
    return text

@mcp.tool("Decode base64", params={"data": "Base64 string"})
def decode_base64(data):
    return data
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	if tools[0].Name != "encode_base64" {
		t.Errorf("tool 0: expected 'encode_base64', got %q", tools[0].Name)
	}
	if tools[1].Name != "decode_base64" {
		t.Errorf("tool 1: expected 'decode_base64', got %q", tools[1].Name)
	}
}

func TestScanDecoratedToolsTypeInference(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Greet", params={"name": "Person name"})
def greet(name, times=3, verbose=False, ratio=0.5):
    return name
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	params := tools[0].Meta.Parameters
	if len(params) != 4 {
		t.Fatalf("expected 4 params, got %d", len(params))
	}

	// name: no default, should be required, type string
	if params[0].Name != "name" || params[0].Type != "string" || !params[0].Required {
		t.Errorf("param 'name': got type=%q required=%v", params[0].Type, params[0].Required)
	}

	// times: default 3 (int), not required, inferred integer
	if params[1].Name != "times" || params[1].Type != "integer" || params[1].Required {
		t.Errorf("param 'times': got type=%q required=%v", params[1].Type, params[1].Required)
	}

	// verbose: default False, not required, inferred boolean
	if params[2].Name != "verbose" || params[2].Type != "boolean" || params[2].Required {
		t.Errorf("param 'verbose': got type=%q required=%v", params[2].Type, params[2].Required)
	}

	// ratio: default 0.5, not required, inferred number
	if params[3].Name != "ratio" || params[3].Type != "number" || params[3].Required {
		t.Errorf("param 'ratio': got type=%q required=%v", params[3].Type, params[3].Required)
	}
}

func TestScanDecoratedToolsExplicitType(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Process", params={
    "items": {"type": "array:string", "description": "Items to process"},
    "count": {"type": "int", "description": "Count", "required": True},
})
def process(items, count=5):
    return items
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	params := tools[0].Meta.Parameters

	// items: explicit type array:string, no default → required
	if params[0].Type != "array:string" {
		t.Errorf("items type: expected 'array:string', got %q", params[0].Type)
	}
	if !params[0].Required {
		t.Error("items should be required (no default)")
	}

	// count: explicit type "int" → normalized to "integer", required override to True
	if params[1].Type != "integer" {
		t.Errorf("count type: expected 'integer', got %q", params[1].Type)
	}
	if !params[1].Required {
		t.Error("count should be required (explicit override)")
	}
}

func TestScanDecoratedToolsParamsMismatch(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Bad tool", params={"nonexistent": "Does not match any param"})
def bad_tool(real_param):
    return real_param
`)

	cfg := testHandlerConfig()
	_, err := ScanDecoratedTools(src, cfg)
	if err == nil {
		t.Fatal("expected error for params key not matching signature")
	}
	if !contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention 'nonexistent': %v", err)
	}
}

func TestScanDecoratedToolsNoParams(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Get time")
def get_time():
    return "now"
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
	if len(tools[0].Meta.Parameters) != 0 {
		t.Errorf("expected 0 params, got %d", len(tools[0].Meta.Parameters))
	}
}

func TestScanDecoratedToolsKeywordsAndDiscoverable(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Search", keywords=["find", "lookup"], discoverable=True)
def search(query):
    return query
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	meta := tools[0].Meta
	if !meta.Discoverable {
		t.Error("expected discoverable=true")
	}
	if len(meta.Keywords) != 2 || meta.Keywords[0] != "find" || meta.Keywords[1] != "lookup" {
		t.Errorf("keywords: got %v", meta.Keywords)
	}
}

func TestScanDecoratedToolsNoDecorators(t *testing.T) {
	// A .py file with no @mcp.tool decorators should produce zero tools.
	src := []byte(`
def helper():
    return "not a tool"

result = helper()
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(tools) != 0 {
		t.Errorf("expected 0 tools, got %d", len(tools))
	}
}

func TestScanToolsFSDualMixed(t *testing.T) {
	// A folder with both legacy (.toml+.py) and decorated (.py only) tools.
	fsys := fstest.MapFS{
		// Legacy tool: greet.toml + greet.py
		"greet.toml": &fstest.MapFile{Data: []byte(`
description = "Greet a person"
[[parameters]]
name = "name"
type = "string"
description = "Person name"
required = true
`)},
		"greet.py": &fstest.MapFile{Data: []byte(`
import scriptling.mcp.tool as tool
name = tool.get_string("name", "World")
tool.return_string("Hello, " + name)
`)},
		// Decorated tool: calc.py (no .toml sibling)
		"calc.py": &fstest.MapFile{Data: []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Calculate", params={"expr": "Expression"})
def calc(expr):
    return expr
`)},
		// Private file (should be skipped)
		"_helpers.py": &fstest.MapFile{Data: []byte(`
def internal():
    pass
`)},
	}

	cfg := testHandlerConfig()
	entries, err := ScanToolsFSDual(fsys, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	// Should find 2 tools: greet (legacy) and calc (decorated)
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Find by name
	var greet, calc *ScannedToolEntry
	for i := range entries {
		switch entries[i].Name {
		case "greet":
			greet = &entries[i]
		case "calc":
			calc = &entries[i]
		}
	}

	if greet == nil {
		t.Fatal("missing 'greet' tool")
	}
	if !greet.Legacy {
		t.Error("greet should be legacy")
	}
	if greet.Meta.Description != "Greet a person" {
		t.Errorf("greet description: %q", greet.Meta.Description)
	}

	if calc == nil {
		t.Fatal("missing 'calc' tool")
	}
	if calc.Legacy {
		t.Error("calc should not be legacy")
	}
	if calc.FuncName != "calc" {
		t.Errorf("calc funcName: %q", calc.FuncName)
	}
	if calc.Meta.Description != "Calculate" {
		t.Errorf("calc description: %q", calc.Meta.Description)
	}
}

func TestScanToolsFSDualSkipsTomlWithoutPy(t *testing.T) {
	// A .toml without a .py sibling — should still be scanned (source nil).
	fsys := fstest.MapFS{
		"orphan.toml": &fstest.MapFile{Data: []byte(`description = "Orphan"`)},
	}

	cfg := testHandlerConfig()
	entries, err := ScanToolsFSDual(fsys, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Source != nil {
		t.Error("expected nil source for orphan toml")
	}
}

// --- helpers ---

func testHandlerConfig() HandlerConfig {
	return NewHandlerConfig(nil)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsCheck(s, substr))
}

func containsCheck(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
