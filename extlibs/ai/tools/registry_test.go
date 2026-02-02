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

# Verify type mappings
assert params["properties"]["str_param"]["type"] == "string"
assert params["properties"]["int_param"]["type"] == "integer"
assert params["properties"]["num_param"]["type"] == "integer"  # number -> integer
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
