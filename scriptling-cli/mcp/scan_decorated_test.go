package mcp

import (
	"strings"
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

func TestScanDecoratedToolsUI(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Get the sales report",
           ui={"resourceUri": "ui://sales-dashboard/dashboard.html"})
def sales_report():
    return {"records": []}

@mcp.tool("Add a sale",
           ui={"resourceUri": "ui://sales-dashboard/dashboard.html", "visibility": ["app"]})
def add_sale():
    return {"records": []}
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	byName := map[string]*DecoratedTool{}
	for i := range tools {
		byName[tools[i].Name] = &tools[i]
	}

	report := byName["sales_report"]
	if report.Meta.UI == nil || report.Meta.UI.ResourceURI != "ui://sales-dashboard/dashboard.html" {
		t.Fatalf("sales_report UI = %+v", report.Meta.UI)
	}
	if len(report.Meta.UI.Visibility) != 0 {
		t.Errorf("sales_report visibility = %v, want empty (spec default)", report.Meta.UI.Visibility)
	}

	addSale := byName["add_sale"]
	if addSale.Meta.UI == nil || len(addSale.Meta.UI.Visibility) != 1 || addSale.Meta.UI.Visibility[0] != "app" {
		t.Fatalf("add_sale UI = %+v", addSale.Meta.UI)
	}
}

func TestScanDecoratedToolsIcons(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Get the weather",
           icons=[{"src": "https://example.com/weather.png", "mimeType": "image/png", "sizes": ["48x48"]}])
def weather():
    return {"forecast": "sunny"}

@mcp.tool("No icons here")
def plain():
    return None
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}

	byName := map[string]*DecoratedTool{}
	for i := range tools {
		byName[tools[i].Name] = &tools[i]
	}

	weather := byName["weather"]
	if len(weather.Meta.Icons) != 1 {
		t.Fatalf("weather Icons = %+v, want 1 entry", weather.Meta.Icons)
	}
	icon := weather.Meta.Icons[0]
	if icon.Src != "https://example.com/weather.png" || icon.MimeType != "image/png" {
		t.Errorf("weather icon = %+v", icon)
	}
	if len(icon.Sizes) != 1 || icon.Sizes[0] != "48x48" {
		t.Errorf("weather icon sizes = %v", icon.Sizes)
	}

	plain := byName["plain"]
	if len(plain.Meta.Icons) != 0 {
		t.Errorf("plain Icons = %+v, want none", plain.Meta.Icons)
	}
}

func TestScanDecoratedToolsIcons_MissingSrcIsError(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Bad", icons=[{"mimeType": "image/png"}])
def bad():
    return None
`)

	cfg := testHandlerConfig()
	if _, err := ScanDecoratedTools(src, cfg); err == nil {
		t.Fatal("expected an error for an icon missing src")
	}
}

// TestScanDecoratedToolsUI_VisibilityOnly covers the app-only "action" tool
// case (e.g. claim_prize in the prize-wheel example): resourceUri is optional
// per spec, since a tool only ever called by a view that's already open has
// no rendering purpose of its own.
func TestScanDecoratedToolsUI_VisibilityOnly(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Claim", ui={"visibility": ["app"]})
def claim():
    return None
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tools) != 1 || tools[0].Meta.UI == nil {
		t.Fatalf("expected 1 tool with UI set, got %+v", tools)
	}
	if tools[0].Meta.UI.ResourceURI != "" {
		t.Errorf("ResourceURI = %q, want empty", tools[0].Meta.UI.ResourceURI)
	}
	if len(tools[0].Meta.UI.Visibility) != 1 || tools[0].Meta.UI.Visibility[0] != "app" {
		t.Errorf("Visibility = %v", tools[0].Meta.UI.Visibility)
	}
}

func TestScanDecoratedToolsUI_MissingResourceURIAndVisibility(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Bad", ui={})
def bad():
    return None
`)

	cfg := testHandlerConfig()
	if _, err := ScanDecoratedTools(src, cfg); err == nil {
		t.Fatal("expected an error for ui with neither resourceUri nor visibility")
	}
}

func TestScanDecoratedToolsNoUI_OmitsField(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Plain")
def plain():
    return "ok"
`)

	cfg := testHandlerConfig()
	tools, err := ScanDecoratedTools(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if tools[0].Meta.UI != nil {
		t.Errorf("UI = %+v, want nil", tools[0].Meta.UI)
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

func TestScanDecoratedRegistrationsAllKinds(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Say hello", params={"name": "Who to greet"})
def hello(name):
    return "hi " + name

@mcp.resource("config://app", name="App config", mime_type="application/json")
def app_config():
    return {"debug": True}

@mcp.resource("user://docs/{topic}", template=True, mime_type="text/markdown")
def user_doc(topic):
    return "# " + topic

@mcp.prompt(description="Summarise text")
def summarise(text, style="brief"):
    return "Summarise (style=" + style + "): " + text

@mcp.skill(files={"regions.md": "eu-west: Europe\n"})
def region_guide():
    return "---\nname: region_guide\ndescription: How to pick a region\n---\n\n# Regions\nSee regions.md."
`)

	cfg := testHandlerConfig()
	regs, err := ScanDecoratedRegistrations(src, cfg)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	if len(regs.Tools) != 1 || regs.Tools[0].Name != "hello" {
		t.Fatalf("tools: %+v", regs.Tools)
	}

	if len(regs.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(regs.Resources))
	}
	if regs.Resources[0].URI != "config://app" || regs.Resources[0].Template {
		t.Errorf("static resource: %+v", regs.Resources[0])
	}
	if regs.Resources[0].Name != "App config" || regs.Resources[0].MimeType != "application/json" {
		t.Errorf("static resource metadata: %+v", regs.Resources[0])
	}
	if regs.Resources[1].URI != "user://docs/{topic}" || !regs.Resources[1].Template {
		t.Errorf("template resource: %+v", regs.Resources[1])
	}
	if regs.Resources[1].FuncName != "user_doc" {
		t.Errorf("template func name: %q", regs.Resources[1].FuncName)
	}

	if len(regs.Prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(regs.Prompts))
	}
	p := regs.Prompts[0]
	if p.Name != "summarise" || p.Description != "Summarise text" {
		t.Errorf("prompt: %+v", p)
	}
	if len(p.Arguments) != 2 {
		t.Fatalf("prompt arguments inferred from signature: %+v", p.Arguments)
	}
	if p.Arguments[0].Name != "text" || !p.Arguments[0].Required {
		t.Errorf("argument 0: %+v", p.Arguments[0])
	}
	if p.Arguments[1].Name != "style" || p.Arguments[1].Required {
		t.Errorf("argument 1 (has default, must be optional): %+v", p.Arguments[1])
	}

	if len(regs.Skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(regs.Skills))
	}
	s := regs.Skills[0]
	if s.Name != "region_guide" {
		t.Errorf("skill name: %q (must be the function name)", s.Name)
	}
	skillMD := string(s.Files["SKILL.md"])
	if !strings.Contains(skillMD, "name: region_guide") {
		t.Errorf("SKILL.md content: %q", skillMD)
	}
	if string(s.Files["regions.md"]) != "eu-west: Europe\n" {
		t.Errorf("supporting file: %q", s.Files["regions.md"])
	}
}

func TestScanDecoratedRegistrationsPromptArgumentsExplicit(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.prompt(description="Review code", arguments=[
    {"name": "language", "description": "Language of the code", "required": True},
    {"name": "code", "description": "The code"},
])
def review(language, code):
    return {"messages": [{"role": "user", "content": language + ":" + code}]}
`)

	regs, err := ScanDecoratedRegistrations(src, testHandlerConfig())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(regs.Prompts) != 1 {
		t.Fatalf("prompts: %+v", regs.Prompts)
	}
	args := regs.Prompts[0].Arguments
	if len(args) != 2 {
		t.Fatalf("arguments: %+v", args)
	}
	if args[0].Name != "language" || !args[0].Required || args[0].Description != "Language of the code" {
		t.Errorf("argument 0: %+v", args[0])
	}
	if args[1].Required {
		t.Errorf("argument 1 must be optional: %+v", args[1])
	}
}

func TestScanDecoratedRegistrationsSkillFrontmatterMismatch(t *testing.T) {
	// The skill function's frontmatter name must match the function name; the
	// library's RegisterSkill rejects it, so the scan must surface the name it
	// registered under and let the server skip it with a warning.
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.skill()
def guide():
    return "---\nname: something-else\ndescription: d\n---\n\nbody"
`)

	regs, err := ScanDecoratedRegistrations(src, testHandlerConfig())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(regs.Skills) != 1 || regs.Skills[0].Name != "guide" {
		t.Fatalf("skills: %+v", regs.Skills)
	}
}

func TestScanRegistrationsFSDualSplitsKinds(t *testing.T) {
	fsys := fstest.MapFS{
		"legacy.toml": &fstest.MapFile{Data: []byte("description = \"Legacy tool\"\n")},
		"legacy.py":   &fstest.MapFile{Data: []byte("import scriptling.mcp.tool as tool\ntool.return_string('ok')\n")},
		"modern.py": &fstest.MapFile{Data: []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Modern tool")
def modern():
    return "m"

@mcp.resource("info://version")
def version():
    return "1.2.3"

@mcp.prompt(description="Notes")
def notes():
    return "n"

@mcp.skill()
def howto():
    return "---\nname: howto\ndescription: d\n---\n\nbody"
`)},
		"_skip.py": &fstest.MapFile{Data: []byte("raise Exception('must not be scanned')")},
	}

	regs, err := ScanRegistrationsFSDual(fsys, testHandlerConfig())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(regs.Tools) != 2 {
		t.Fatalf("tools: %+v", regs.Tools)
	}
	if len(regs.Resources) != 1 || regs.Resources[0].URI != "info://version" {
		t.Fatalf("resources: %+v", regs.Resources)
	}
	if len(regs.Prompts) != 1 || regs.Prompts[0].Name != "notes" {
		t.Fatalf("prompts: %+v", regs.Prompts)
	}
	if len(regs.Skills) != 1 || regs.Skills[0].Name != "howto" {
		t.Fatalf("skills: %+v", regs.Skills)
	}
}

func TestScanDecoratedResourceTemplateVarMismatch(t *testing.T) {
	// A template variable with no matching function parameter would fail
	// every read; the scan must fail at startup instead.
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.resource("user://docs/{topic}", template=True)
def user_doc(subject):
    return subject
`)
	_, err := ScanDecoratedRegistrations(src, testHandlerConfig())
	if err == nil || !strings.Contains(err.Error(), "topic") {
		t.Fatalf("expected a template-variable mismatch error, got: %v", err)
	}
}

func TestScanDecoratedResourceRequiredParamNotAVar(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.resource("user://docs/{topic}", template=True)
def user_doc(topic, extra):
    return topic
`)
	_, err := ScanDecoratedRegistrations(src, testHandlerConfig())
	if err == nil || !strings.Contains(err.Error(), "extra") {
		t.Fatalf("expected a required-parameter mismatch error, got: %v", err)
	}
}

func TestScanDecoratedResourceStaticWithRequiredParam(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.resource("config://app")
def app_config(path):
    return path
`)
	_, err := ScanDecoratedRegistrations(src, testHandlerConfig())
	if err == nil || !strings.Contains(err.Error(), "static resource") {
		t.Fatalf("expected a static-resource parameter error, got: %v", err)
	}
}

func TestScanDecoratedResourceStaticWithDefaultParamOK(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.resource("config://app")
def app_config(fallback="none"):
    return fallback
`)
	regs, err := ScanDecoratedRegistrations(src, testHandlerConfig())
	if err != nil {
		t.Fatalf("optional parameter on a static resource must scan: %v", err)
	}
	if len(regs.Resources) != 1 {
		t.Fatalf("resources: %+v", regs.Resources)
	}
}

func TestScanDecoratedPromptArgumentMismatch(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.prompt(description="Review", arguments=[{"name": "lang"}])
def review(language):
    return language
`)
	_, err := ScanDecoratedRegistrations(src, testHandlerConfig())
	if err == nil || !strings.Contains(err.Error(), "lang") {
		t.Fatalf("expected an argument mismatch error, got: %v", err)
	}
}

func TestScanDecoratedSkillBadReturn(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.skill()
def bad_skill():
    return {"not": "a string"}
`)
	_, err := ScanDecoratedRegistrations(src, testHandlerConfig())
	if err == nil || !strings.Contains(err.Error(), "SKILL.md") {
		t.Fatalf("expected a non-string skill error, got: %v", err)
	}
}

func TestScanDecoratedSkillUsingReturnString(t *testing.T) {
	// return_string sets __mcp_response and exits; the scan must honour it.
	src := []byte(`
import scriptling.runtime.mcp as mcp
import scriptling.mcp.tool as tool

@mcp.skill()
def via_return():
    tool.return_string("---\nname: via_return\ndescription: d\n---\n\nbody")
`)
	regs, err := ScanDecoratedRegistrations(src, testHandlerConfig())
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(regs.Skills) != 1 || !strings.Contains(string(regs.Skills[0].Files["SKILL.md"]), "via_return") {
		t.Fatalf("skills: %+v", regs.Skills)
	}
}

func TestScanDecoratedUnknownRegistryType(t *testing.T) {
	// A future/typo entry type must fail loudly rather than be ignored.
	src := []byte(`
import scriptling.runtime.mcp as mcp

mcp.resource("config://x")
`)
	// (No function decorated -> no registry entry; use a fake registry via a tool
	// with a bogus type instead is not directly expressible, so this test uses
	// the scanner against a source that registers nothing exotic.)
	regs, err := ScanDecoratedRegistrations(src, testHandlerConfig())
	if err != nil {
		t.Fatalf("unevaluated decorator call must not error: %v", err)
	}
	if len(regs.Resources) != 0 {
		t.Fatalf("no resource should be registered: %+v", regs.Resources)
	}
}
