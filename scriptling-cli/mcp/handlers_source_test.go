package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/paularlott/logger"
	mcplib "github.com/paularlott/mcp"
)

// TestBuildToolHandlerSource verifies a tool handler built from in-memory
// source (no disk) executes and returns the script response.
func TestBuildToolHandlerSource(t *testing.T) {
	src := []byte("import scriptling.mcp.tool as tool\ntool.return_string('hi ' + tool.get_string('who'))\n")
	cfg := NewHandlerConfig(nil, WithLogger(logger.NewNullLogger()))
	handler := BuildToolHandlerSource(src, cfg)

	resp, err := handler(context.Background(), mcplib.NewToolRequest(map[string]any{"who": "pack"}))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.Content) != 1 || resp.Content[0].Text != "hi pack" {
		t.Fatalf("expected 'hi pack', got %+v", resp.Content)
	}
}

// TestBuildToolHandlerSourceError verifies return_error propagates as an error.
func TestBuildToolHandlerSourceError(t *testing.T) {
	src := []byte("import scriptling.mcp.tool as tool\ntool.return_error('nope')\n")
	handler := BuildToolHandlerSource(src, NewHandlerConfig(nil))

	if _, err := handler(context.Background(), mcplib.NewToolRequest(nil)); err == nil {
		t.Fatal("expected error from return_error")
	}
}

// TestBuildResourceScriptHandlerSource verifies a resource template script runs
// from in-memory source and receives template vars.
func TestBuildResourceScriptHandlerSource(t *testing.T) {
	src := []byte("import scriptling.mcp.tool as tool\ntool.return_string('key=' + tool.get_string('key'))\n")
	handler := BuildResourceScriptHandlerSource(src, "text/plain", NewHandlerConfig(nil))

	req := mcplib.NewResourceRequest("kv://answer", map[string]string{"key": "answer"})
	resp, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.Contents) != 1 || resp.Contents[0].Text != "key=answer" {
		t.Fatalf("expected key=answer, got %+v", resp.Contents)
	}
}

// TestBuildPromptScriptHandlerSource verifies a prompt script runs from
// in-memory source and its response is decoded.
func TestBuildPromptScriptHandlerSource(t *testing.T) {
	src := []byte("import scriptling.mcp.tool as tool\ntool.return_string('do the thing')")
	handler := BuildPromptScriptHandlerSource(src, NewHandlerConfig(nil))

	resp, err := handler(context.Background(), mcplib.NewPromptRequest(nil))
	if err != nil {
		t.Fatalf("handler: %v", err)
	}
	if len(resp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %+v", resp.Messages)
	}
}

// TestStaticHandlersFromMemory verifies the read-func based static handlers.
func TestStaticHandlersFromMemory(t *testing.T) {
	rh := BuildStaticResourceHandler(func() ([]byte, error) { return []byte("res data"), nil }, "memo://x", "text/plain")
	resp, err := rh(context.Background(), mcplib.NewResourceRequest("memo://x", nil))
	if err != nil {
		t.Fatalf("resource handler: %v", err)
	}
	if len(resp.Contents) != 1 || resp.Contents[0].Text != "res data" {
		t.Fatalf("expected res data, got %+v", resp.Contents)
	}

	ph := BuildStaticPromptHandler(func() ([]byte, error) { return []byte("prompt body"), nil })
	presp, err := ph(context.Background(), mcplib.NewPromptRequest(nil))
	if err != nil {
		t.Fatalf("prompt handler: %v", err)
	}
	if len(presp.Messages) != 1 {
		t.Fatalf("expected 1 message, got %+v", presp.Messages)
	}

	// Read failure surfaces as an error.
	boom := func() ([]byte, error) { return nil, errors.New("gone") }
	if _, err := BuildStaticResourceHandler(boom, "memo://x", "")(context.Background(), mcplib.NewResourceRequest("memo://x", nil)); err == nil {
		t.Error("expected resource read error")
	}
	if _, err := BuildStaticPromptHandler(boom)(context.Background(), mcplib.NewPromptRequest(nil)); err == nil {
		t.Error("expected prompt read error")
	}
}

// TestBuildToolHandlerSourceNoSiblingImports documents that source-built
// handlers do not get any implicit library dir.
func TestBuildToolHandlerSourceNoSiblingImports(t *testing.T) {
	src := []byte("import nonexistent_helper_xyz\n")
	handler := BuildToolHandlerSource(src, NewHandlerConfig(nil))
	_, err := handler(context.Background(), mcplib.NewToolRequest(nil))
	if err == nil || !strings.Contains(err.Error(), "nonexistent_helper_xyz") {
		t.Fatalf("expected import failure mentioning module, got %v", err)
	}
}
