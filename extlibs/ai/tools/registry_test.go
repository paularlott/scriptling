package tools_test

import (
	"testing"

	scriptlib "github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/ai"
	"github.com/paularlott/scriptling/stdlib"
)

func TestToolsRegistry(t *testing.T) {
	script := `
import scriptling.ai as ai

# Create registry
registry = ai.ToolRegistry()

# Add tools
def read_func(args):
    return "file content"

def write_func(args):
    return "ok"

registry.add("read_file", "Read a file", {"path": "string", "limit": "integer?"}, read_func)
registry.add("write_file", "Write a file", {"path": "string", "content": "string"}, write_func)

# Build schemas
schemas = registry.build()

# Verify schemas
assert len(schemas) == 2
assert schemas[0]["type"] == "function"
assert schemas[0]["function"]["name"] == "read_file"
assert schemas[0]["function"]["description"] == "Read a file"
assert schemas[0]["function"]["parameters"]["type"] == "object"
assert "path" in schemas[0]["function"]["parameters"]["properties"]
assert "limit" in schemas[0]["function"]["parameters"]["properties"]
assert schemas[0]["function"]["parameters"]["required"] == ["path"]

# Test get_handler
handler = registry.get_handler("read_file")
result = handler({"path": "test.txt"})
assert result == "file content"

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

func TestToolsTypeMapping(t *testing.T) {
	script := `
import scriptling.ai as ai

registry = ai.ToolRegistry()

def dummy(args):
    return "ok"

registry.add("test", "Test tool", {
    "str_param": "string",
    "int_param": "integer",
    "num_param": "number",
    "bool_param": "boolean",
    "arr_param": "array",
    "obj_param": "object",
    "opt_param": "string?"
}, dummy)

schemas = registry.build()
params = schemas[0]["function"]["parameters"]

# Canonical JSON Schema types pass through unchanged.
assert params["properties"]["str_param"]["type"] == "string"
assert params["properties"]["int_param"]["type"] == "integer"
assert params["properties"]["num_param"]["type"] == "number"
assert params["properties"]["bool_param"]["type"] == "boolean"
assert params["properties"]["arr_param"]["type"] == "array"
assert params["properties"]["obj_param"]["type"] == "object"
assert params["properties"]["opt_param"]["type"] == "string"

# Verify required list
required = params["required"]
assert "str_param" in required
assert "int_param" in required
assert "opt_param" not in required

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestToolsTypeAliases verifies that Python-style type names are accepted
// and mapped to their canonical JSON Schema equivalents.
func TestToolsTypeAliases(t *testing.T) {
	script := `
import scriptling.ai as ai

registry = ai.ToolRegistry()

def dummy(args):
    return "ok"

registry.add("aliases", "Aliases tool", {
    "a": "int",
    "b": "float",
    "c": "str",
    "d": "bool",
    "e": "dict",
    "f": "list",
    "g": "int?",
    "h": "float?",
    "i": "bool?",
}, dummy)

schemas = registry.build()
props = schemas[0]["function"]["parameters"]["properties"]

assert props["a"]["type"] == "integer"
assert props["b"]["type"] == "number"
assert props["c"]["type"] == "string"
assert props["d"]["type"] == "boolean"
assert props["e"]["type"] == "object"
assert props["f"]["type"] == "array"
assert props["g"]["type"] == "integer"
assert props["h"]["type"] == "number"
assert props["i"]["type"] == "boolean"

required = schemas[0]["function"]["parameters"]["required"]
assert "a" in required
assert "b" in required
assert "g" not in required
assert "h" not in required
assert "i" not in required

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestToolsNumberPreserved ensures "number" is emitted as "number" rather
// than being silently downgraded to "integer".
func TestToolsNumberPreserved(t *testing.T) {
	script := `
import scriptling.ai as ai

registry = ai.ToolRegistry()

registry.add("calc", "Calc tool", {"value": "number"}, lambda args: args.get("value", 0))
schemas = registry.build()
assert schemas[0]["function"]["parameters"]["properties"]["value"]["type"] == "number"

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestToolsUnknownType verifies that an unknown type string is rejected at
// add() time so the caller gets a clear error rather than invalid schema
// output at build() time.
func TestToolsUnknownType(t *testing.T) {
	script := `
import scriptling.ai as ai

registry = ai.ToolRegistry()

err = None
try:
    registry.add("bad", "Bad tool", {"x": "notatype"}, lambda args: None)
except Exception as e:
    err = str(e)

assert err is not None, "expected an error for unknown type"
assert "notatype" in err, "error should reference the bad type, got: " + str(err)

# Ensure the bad tool was not stored.
schemas = registry.build()
assert len(schemas) == 0, "no tools should be registered when add() fails"

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestToolsAddSchema verifies add_schema: the full JSON Schema is emitted
// verbatim (nested properties survive), the handler dispatches like any other
// tool, and a duplicate name is an error.
func TestToolsAddSchema(t *testing.T) {
	script := `
import scriptling.ai as ai

registry = ai.ToolRegistry()

def search(args):
    return "results for " + args["query"]

registry.add_schema("shop__search", "Search products", {
    "type": "object",
    "properties": {
        "query": {"type": "string", "description": "Search terms"},
        "filters": {
            "type": "object",
            "properties": {"tag": {"type": "string", "enum": ["new", "sale"]}},
        },
    },
    "required": ["query"],
}, search)

# A flat add() tool can coexist with add_schema tools.
registry.add("local", "Local tool", {}, lambda args: "local result")

schemas = registry.build()
assert len(schemas) == 2

params = schemas[0]["function"]["parameters"]
assert params["type"] == "object"
assert params["required"] == ["query"]
assert params["properties"]["query"]["description"] == "Search terms"
assert params["properties"]["filters"]["properties"]["tag"]["enum"] == ["new", "sale"]

handler = registry.get_handler("shop__search")
assert handler({"query": "mug"}) == "results for mug"
assert registry.get_handler("local")({}) == "local result"

err = None
try:
    registry.add_schema("shop__search", "Duplicate", {"type": "object"}, lambda args: None)
except Exception as e:
    err = str(e)
assert err is not None, "duplicate add_schema must error"
assert "shop__search" in err, "error should name the duplicate, got: " + str(err)

# The failed duplicate must not have been stored.
assert len(registry.build()) == 2

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}

	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}

// TestToolsAddSchemaNonDict verifies a non-dict schema is rejected at
// registration time with a clear error.
func TestToolsAddSchemaNonDict(t *testing.T) {
	script := `
import scriptling.ai as ai

registry = ai.ToolRegistry()
err = None
try:
    registry.add_schema("bad", "Bad", ["not", "a", "dict"], lambda args: None)
except Exception as e:
    err = str(e)
assert err is not None, "non-dict schema must error"
assert "dict" in err.lower(), "error should mention dict, got: " + str(err)
assert len(registry.build()) == 0, "failed add must not store the tool"

"OK"
`

	p := scriptlib.New()
	stdlib.RegisterAll(p)
	ai.Register(p)

	result, err := p.Eval(script)
	if err != nil {
		t.Fatalf("Script failed: %v", err)
	}
	if str, err := result.AsString(); err != nil || str != "OK" {
		t.Fatalf("Expected 'OK', got: %v (err: %v)", result, err)
	}
}
