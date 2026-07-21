package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/paularlott/mcp"
)

func TestBuildToolHandlerFuncStringReturn(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Greet", params={"name": "Person name"})
def greet(name):
    return "Hello, " + name + "!"
`)

	cfg := testHandlerConfig()
	handler := BuildToolHandlerFunc(src, "greet", cfg)

	req := newToolRequest(t, map[string]interface{}{"name": "Alice"})
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatal("empty response")
	}
	if resp.Content[0].Text != "Hello, Alice!" {
		t.Errorf("expected 'Hello, Alice!', got %q", resp.Content[0].Text)
	}
}

func TestBuildToolHandlerFuncDictReturn(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Info")
def info():
    return {"status": "ok", "count": 42}
`)

	cfg := testHandlerConfig()
	handler := BuildToolHandlerFunc(src, "info", cfg)

	req := newToolRequest(t, map[string]interface{}{})
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatal("empty response")
	}

	// Should be valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(resp.Content[0].Text), &result); err != nil {
		t.Fatalf("response is not valid JSON: %v, text=%q", err, resp.Content[0].Text)
	}
	if result["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", result["status"])
	}
	// JSON numbers come back as float64
	if result["count"] != float64(42) {
		t.Errorf("expected count 42, got %v", result["count"])
	}
}

func TestBuildToolHandlerFuncNoneReturn(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Noop")
def noop():
    pass
`)

	cfg := testHandlerConfig()
	handler := BuildToolHandlerFunc(src, "noop", cfg)

	req := newToolRequest(t, map[string]interface{}{})
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp == nil || len(resp.Content) == 0 {
		t.Fatal("empty response")
	}
	if resp.Content[0].Text != "" {
		t.Errorf("expected empty text, got %q", resp.Content[0].Text)
	}
}

func TestBuildToolHandlerFuncIntReturn(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Add", params={"a": "First", "b": "Second"})
def add(a, b):
    return a + b
`)

	cfg := testHandlerConfig()
	handler := BuildToolHandlerFunc(src, "add", cfg)

	req := newToolRequest(t, map[string]interface{}{"a": 3, "b": 4})
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Content[0].Text != "7" {
		t.Errorf("expected '7', got %q", resp.Content[0].Text)
	}
}

func TestBuildToolHandlerFuncExceptionReturn(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Fail")
def fail():
    raise ValueError("something went wrong")
`)

	cfg := testHandlerConfig()
	handler := BuildToolHandlerFunc(src, "fail", cfg)

	req := newToolRequest(t, map[string]interface{}{})
	_, err := handler(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from handler")
	}
	if !containsCheck(err.Error(), "something went wrong") {
		t.Errorf("error should contain 'something went wrong': %v", err)
	}
}

func TestBuildToolHandlerFuncWithDefaults(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Repeat", params={"text": "Text", "times": {"type": "int", "description": "Count"}})
def repeat(text, times=3):
    return text * times
`)

	cfg := testHandlerConfig()
	handler := BuildToolHandlerFunc(src, "repeat", cfg)

	// Call without 'times' — should use default
	req := newToolRequest(t, map[string]interface{}{"text": "ab"})
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if resp.Content[0].Text != "ababab" {
		t.Errorf("expected 'ababab', got %q", resp.Content[0].Text)
	}

	// Call with 'times' = 2
	req2 := newToolRequest(t, map[string]interface{}{"text": "x", "times": 2})
	resp2, err2 := handler(context.Background(), req2)
	if err2 != nil {
		t.Fatalf("handler error: %v", err2)
	}
	if resp2.Content[0].Text != "xx" {
		t.Errorf("expected 'xx', got %q", resp2.Content[0].Text)
	}
}

func TestBuildToolHandlerFuncListReturn(t *testing.T) {
	src := []byte(`
import scriptling.runtime.mcp as mcp

@mcp.tool("Numbers")
def numbers():
    return [1, 2, 3]
`)

	cfg := testHandlerConfig()
	handler := BuildToolHandlerFunc(src, "numbers", cfg)

	req := newToolRequest(t, map[string]interface{}{})
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var result []interface{}
	if err := json.Unmarshal([]byte(resp.Content[0].Text), &result); err != nil {
		t.Fatalf("not valid JSON: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 items, got %d", len(result))
	}
}

// --- helpers ---

// newToolRequest creates a ToolRequest with the given arguments.
func newToolRequest(t *testing.T, args map[string]interface{}) *mcplib.ToolRequest {
	t.Helper()
	req := mcplib.NewToolRequest(args)
	return req
}
