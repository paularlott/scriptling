package mcp

import (
	"testing"

	"github.com/paularlott/scriptling/object"

	mcplib "github.com/paularlott/mcp"
)

// A tool dict from list_tools carries is_app: true only for a tool linked
// to a ui:// resource (an MCP Apps view), so scripts can tell app tools
// from plain ones.
func TestConvertToolsToListMarksApps(t *testing.T) {
	tools := []mcplib.MCPTool{
		{Name: "plain_tool", Description: "a plain tool"},
		{Name: "wheel", Description: "an app", Meta: map[string]any{"ui": map[string]any{"resourceUri": "ui://x/wheel.html"}}},
	}

	list := convertToolsToList(tools)
	elems := list.(*object.List).Elements
	if len(elems) != 2 {
		t.Fatalf("elements = %d, want 2", len(elems))
	}

	isApp := func(i int) bool {
		dict, ok := elems[i].(*object.Dict)
		if !ok {
			t.Fatalf("element %d is not a dict", i)
		}
		pair, ok := dict.GetByString("is_app")
		if !ok {
			t.Fatalf("element %d has no is_app key", i)
		}
		b, ok := pair.Value.(*object.Boolean)
		if !ok {
			t.Fatalf("is_app is not a boolean: %T", pair.Value)
		}
		return b.BoolValue()
	}

	if isApp(0) {
		t.Error("plain tool must not be marked is_app")
	}
	if !isApp(1) {
		t.Error("app tool (ui.resourceUri set) must be marked is_app")
	}
}
