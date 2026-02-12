package mcp_test

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/extlibs/mcp"
)

// TestRunToolScriptAllTypes validates RunToolScript with all MCP data types
func TestRunToolScriptAllTypes(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_string, get_int, get_float, get_bool, get_string_list, get_int_list, get_float_list, get_bool_list, return_object

# Basic types
name = get_string("name", "guest")
age = get_int("age", 0)
price = get_float("price", 0.0)
enabled = get_bool("enabled", False)

# Array types
tags = get_string_list("tags")
ids = get_int_list("ids")
prices = get_float_list("prices")
flags = get_bool_list("flags")

return_object({
    "name": name,
    "age": age,
    "price": price,
    "enabled": enabled,
    "tags": tags,
    "ids": ids,
    "prices": prices,
    "flags": flags
})
`

	params := map[string]interface{}{
		"name":    "Alice",
		"age":     30,
		"price":   19.99,
		"enabled": true,
		"tags":    []string{"tag1", "tag2", "tag3"},
		"ids":     []int{1, 2, 3},
		"prices":  []float64{10.5, 20.5, 30.5},
		"flags":   []bool{true, false, true},
	}

	response, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if response == "" {
		t.Errorf("Expected non-empty response")
	}
}

// TestRunToolScriptStringList validates string array handling
func TestRunToolScriptStringList(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_string_list, return_object

args = get_string_list("args")
return_object({"count": len(args), "first": args[0] if args else None})
`

	params := map[string]interface{}{
		"args": []string{"--verbose", "-o", "file.txt"},
	}

	response, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if response == "" {
		t.Errorf("Expected non-empty response")
	}
}

// TestRunToolScriptIntList validates integer array handling
func TestRunToolScriptIntList(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_int_list, return_object

ids = get_int_list("ids")
total = sum(ids)
return_object({"count": len(ids), "sum": total})
`

	params := map[string]interface{}{
		"ids": []int{1, 2, 3, 4, 5},
	}

	response, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if response == "" {
		t.Errorf("Expected non-empty response")
	}
}

// TestRunToolScriptFloatList validates float array handling
func TestRunToolScriptFloatList(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_float_list, return_object

prices = get_float_list("prices")
total = sum(prices)
return_object({"count": len(prices), "total": total})
`

	params := map[string]interface{}{
		"prices": []float64{19.99, 29.99, 39.99},
	}

	response, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if response == "" {
		t.Errorf("Expected non-empty response")
	}
}

// TestRunToolScriptBoolList validates boolean array handling
func TestRunToolScriptBoolList(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_bool_list, return_object

flags = get_bool_list("flags")
true_count = sum(1 for f in flags if f)
return_object({"count": len(flags), "true_count": true_count})
`

	params := map[string]interface{}{
		"flags": []bool{true, false, true, true, false},
	}

	response, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if response == "" {
		t.Errorf("Expected non-empty response")
	}
}

// TestRunToolScriptMixedTypes validates mixed type handling
func TestRunToolScriptMixedTypes(t *testing.T) {
	sl := scriptling.New()
	mcp.RegisterToolHelpers(sl)

	script := `
from scriptling.mcp.tool import get_string, get_int, get_string_list, return_object

name = get_string("name")
count = get_int("count")
items = get_string_list("items")

return_object({
    "message": f"{name} has {count} items",
    "items": items
})
`

	params := map[string]interface{}{
		"name":  "Alice",
		"count": 3,
		"items": []string{"apple", "banana", "cherry"},
	}

	response, exitCode, err := mcp.RunToolScript(context.Background(), sl, script, params)
	if err != nil {
		t.Fatalf("RunToolScript failed: %v", err)
	}
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}
	if response == "" {
		t.Errorf("Expected non-empty response")
	}
}
