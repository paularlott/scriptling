package extlibs_test

import (
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
    return str(eval(expr))
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
