package extlibs_test

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/extlibs/mcp"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// newTestScriptling creates a Scriptling instance with the runtime.mcp library
// registered (matching the real server setup).
func newTestScriptling() *scriptling.Scriptling {
	p := scriptling.New()
	stdlib.RegisterAll(p)
	extlibs.RegisterRuntimeLibraryAll(p, nil)
	mcp.Register(p)
	return p
}

func TestMCPToolDecoratorBasic(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Calculate an expression", params={"expr": "Math expression"})
def calc(expr):
    return str(int(expr) * 2)
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	registry := getMCPRegistry(t, p)
	if len(registry) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(registry))
	}

	entry := registry[0]
	assertDictString(t, entry, "name", "calc")
	assertDictString(t, entry, "description", "Calculate an expression")
	assertDictBool(t, entry, "discoverable", false)
}

func TestMCPToolDecoratorMultiple(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Encode base64", params={"text": "Text to encode"})
def encode(text):
    return text

@mcp.tool("Decode base64", params={"data": "Base64 data"})
def decode(data):
    return data
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	registry := getMCPRegistry(t, p)
	if len(registry) != 2 {
		t.Fatalf("expected 2 registrations, got %d", len(registry))
	}

	assertDictString(t, registry[0], "name", "encode")
	assertDictString(t, registry[1], "name", "decode")
}

func TestMCPToolDecoratorKeywords(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Search", keywords=["find", "lookup"], discoverable=True)
def search(query):
    return query
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	registry := getMCPRegistry(t, p)
	if len(registry) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(registry))
	}

	entry := registry[0]
	assertDictBool(t, entry, "discoverable", true)

	// Verify keywords list
	kwPair, ok := entry.GetByString("keywords")
	if !ok {
		t.Fatal("missing 'keywords' in entry")
	}
	kwList, ok := kwPair.Value.(*object.List)
	if !ok {
		t.Fatalf("keywords is not a list, got %T", kwPair.Value)
	}
	if len(kwList.Elements) != 2 {
		t.Fatalf("expected 2 keywords, got %d", len(kwList.Elements))
	}
}

func TestMCPToolDecoratorParamsDict(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Greet", params={
    "name": "Person to greet",
    "times": {"type": "int", "description": "Repeat count"},
})
def greet(name, times=1):
    return name
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	registry := getMCPRegistry(t, p)
	if len(registry) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(registry))
	}

	// Verify params dict is stored
	entry := registry[0]
	paramsPair, ok := entry.GetByString("params")
	if !ok {
		t.Fatal("missing 'params' in entry")
	}
	paramsDict, ok := paramsPair.Value.(*object.Dict)
	if !ok {
		t.Fatalf("params is not a dict, got %T", paramsPair.Value)
	}

	// Check "name" entry is a string
	namePair, ok := paramsDict.GetByString("name")
	if !ok {
		t.Fatal("missing 'name' in params")
	}
	nameStr, nameErr := namePair.Value.AsString()
	if nameErr != nil {
		t.Fatalf("params.name is not a string: %v", nameErr)
	}
	if nameStr != "Person to greet" {
		t.Errorf("expected 'Person to greet', got %q", nameStr)
	}

	// Check "times" entry is a dict with type and description
	timesPair, ok := paramsDict.GetByString("times")
	if !ok {
		t.Fatal("missing 'times' in params")
	}
	timesDict, ok := timesPair.Value.(*object.Dict)
	if !ok {
		t.Fatalf("params.times is not a dict, got %T", timesPair.Value)
	}
	typePair, ok := timesDict.GetByString("type")
	if !ok {
		t.Fatal("missing 'type' in params.times")
	}
	typeStr, _ := typePair.Value.AsString()
	if typeStr != "int" {
		t.Errorf("expected type 'int', got %q", typeStr)
	}
}

func TestMCPToolDecoratorNoParams(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Get server time")
def server_time():
    return "now"
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	registry := getMCPRegistry(t, p)
	if len(registry) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(registry))
	}

	entry := registry[0]
	assertDictString(t, entry, "name", "server_time")
	// params should not be present when not supplied
	if _, ok := entry.GetByString("params"); ok {
		t.Error("params should not be present when not supplied")
	}
}

func TestMCPToolDecoratorFunctionStillCallable(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Add two numbers", params={"a": "First", "b": "Second"})
def add(a, b):
    return a + b

result = add(3, 4)
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	result, getErr := p.GetVar("result")
	if getErr != nil {
		t.Fatalf("failed to get result: %v", getErr)
	}
	if result != int64(7) {
		t.Errorf("expected 7, got %v", result)
	}
}

func TestMCPToolDecoratorNonFunctionError(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

decorator = mcp.tool("test")
result = decorator("not a function")
`)
	if err == nil {
		t.Fatal("expected error when decorating a non-function")
	}
}

func TestMCPToolDecoratorUI(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Get the sales report",
           ui={"resourceUri": "ui://sales-dashboard/dashboard.html", "visibility": ["model", "app"]})
def sales_report():
    return {"records": []}
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	registry := getMCPRegistry(t, p)
	if len(registry) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(registry))
	}

	uiPair, ok := registry[0].GetByString("ui")
	if !ok {
		t.Fatal("missing 'ui' in entry")
	}
	uiDict, ok := uiPair.Value.(*object.Dict)
	if !ok {
		t.Fatalf("'ui' type = %T, want *object.Dict", uiPair.Value)
	}
	assertDictString(t, uiDict, "resourceUri", "ui://sales-dashboard/dashboard.html")
}

func TestMCPToolDecoratorIcons(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Get the weather",
           icons=[{"src": "https://example.com/weather.png", "mimeType": "image/png"}])
def weather():
    return {"forecast": "sunny"}
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	registry := getMCPRegistry(t, p)
	if len(registry) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(registry))
	}

	iconsPair, ok := registry[0].GetByString("icons")
	if !ok {
		t.Fatal("missing 'icons' in entry")
	}
	iconsList, ok := iconsPair.Value.(*object.List)
	if !ok || len(iconsList.Elements) != 1 {
		t.Fatalf("'icons' = %#v, want a 1-element list", iconsPair.Value)
	}
	iconDict, ok := iconsList.Elements[0].(*object.Dict)
	if !ok {
		t.Fatalf("icons[0] type = %T, want *object.Dict", iconsList.Elements[0])
	}
	assertDictString(t, iconDict, "src", "https://example.com/weather.png")
}

func TestMCPToolDecoratorIcons_NotListIsError(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Bad tool", icons="https://example.com/icon.png")
def bad():
    return None
`)
	if err == nil {
		t.Fatal("expected an error when icons is not a list")
	}
}

func TestMCPToolDecoratorUI_NotDictIsError(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Bad tool", ui="ui://not-a-dict")
def bad():
    return None
`)
	if err == nil {
		t.Fatal("expected an error when ui is not a dict")
	}
}

func TestMCPToolDecoratorViaParentImport(t *testing.T) {
	// Test that runtime.mcp.tool works via `import scriptling.runtime as runtime`
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime as runtime

@runtime.mcp.tool("A tool")
def my_tool():
    return "done"
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	registry := getMCPRegistry(t, p)
	if len(registry) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(registry))
	}
	assertDictString(t, registry[0], "name", "my_tool")
}

// --- helpers ---

func getMCPRegistry(t *testing.T, p *scriptling.Scriptling) []*object.Dict {
	t.Helper()
	obj, err := p.GetVarAsObject(extlibs.MCPRegistryVar)
	if err != nil {
		t.Fatalf("failed to get %s: %v", extlibs.MCPRegistryVar, err)
	}
	list, ok := obj.(*object.List)
	if !ok {
		t.Fatalf("%s is not a list, got %T", extlibs.MCPRegistryVar, obj)
	}
	var entries []*object.Dict
	for _, elem := range list.Elements {
		d, ok := elem.(*object.Dict)
		if !ok {
			t.Fatalf("registry entry is not a dict, got %T", elem)
		}
		entries = append(entries, d)
	}
	return entries
}

func assertDictString(t *testing.T, d *object.Dict, key, expected string) {
	t.Helper()
	pair, ok := d.GetByString(key)
	if !ok {
		t.Fatalf("missing key %q in dict", key)
	}
	got, err := pair.Value.AsString()
	if err != nil {
		t.Fatalf("key %q is not a string: %v", key, err)
	}
	if got != expected {
		t.Errorf("key %q: expected %q, got %q", key, expected, got)
	}
}

func assertDictBool(t *testing.T, d *object.Dict, key string, expected bool) {
	t.Helper()
	pair, ok := d.GetByString(key)
	if !ok {
		t.Fatalf("missing key %q in dict", key)
	}
	got, err := pair.Value.AsBool()
	if err != nil {
		t.Fatalf("key %q is not a bool: %v", key, err)
	}
	if got != expected {
		t.Errorf("key %q: expected %v, got %v", key, expected, got)
	}
}

// TestMCPResourcePromptSkillDecorators covers the non-tool registration
// decorators recording their entries in __mcp_registry with the right type
// and fields, plus their validation errors.
func TestMCPResourcePromptSkillDecorators(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.resource("config://app", name="App config", mime_type="application/json")
def app_config():
    return {"debug": True}

@mcp.resource("user://docs/{topic}", template=True, mime_type="text/markdown")
def user_doc(topic):
    return "# " + topic

@mcp.prompt(description="Summarise")
def summarise(text, style="brief"):
    return text

@mcp.skill(files={"extra.md": "x"})
def guide():
    return "body"
`)
	if err != nil {
		t.Fatalf("eval error: %v", err)
	}

	registry := getMCPRegistry(t, p)
	if len(registry) != 4 {
		t.Fatalf("expected 4 registrations, got %d", len(registry))
	}

	res := registry[0]
	assertDictString(t, res, "type", "resource")
	assertDictString(t, res, "uri", "config://app")
	assertDictString(t, res, "name", "App config")
	assertDictString(t, res, "mime_type", "application/json")
	assertDictString(t, res, "func", "app_config")
	assertDictBool(t, res, "template", false)

	tmpl := registry[1]
	assertDictString(t, tmpl, "uri", "user://docs/{topic}")
	assertDictBool(t, tmpl, "template", true)

	prm := registry[2]
	assertDictString(t, prm, "type", "prompt")
	assertDictString(t, prm, "name", "summarise")
	assertDictString(t, prm, "func", "summarise")

	skl := registry[3]
	assertDictString(t, skl, "type", "skill")
	assertDictString(t, skl, "name", "guide")
	assertDictString(t, skl, "func", "guide")
}

// TestMCPSkillDecoratorFilesNotDict: a non-dict files argument errors at the
// decorator call.
func TestMCPSkillDecoratorFilesNotDict(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.skill(files=["not", "a", "dict"])
def guide():
    return "body"
`)
	if err == nil || !strings.Contains(err.Error(), "files") {
		t.Fatalf("expected a files type error, got: %v", err)
	}
}

// TestMCPPromptDecoratorArgumentsNotList: a non-list arguments errors at the
// decorator call.
func TestMCPPromptDecoratorArgumentsNotList(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.prompt(description="d", arguments={"name": "x"})
def pr(x):
    return x
`)
	if err == nil || !strings.Contains(err.Error(), "arguments") {
		t.Fatalf("expected an arguments type error, got: %v", err)
	}
}

// TestMCPResourceDecoratorEmptyURI: an empty uri errors at the decorator call.
func TestMCPResourceDecoratorEmptyURI(t *testing.T) {
	p := newTestScriptling()
	_, err := p.Eval(`
import scriptling.runtime.mcp as mcp

@mcp.resource("")
def broken():
    return "x"
`)
	if err == nil || !strings.Contains(err.Error(), "uri") {
		t.Fatalf("expected a uri error, got: %v", err)
	}
}
