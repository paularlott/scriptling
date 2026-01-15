package ai

import (
	"context"
	"testing"

	"github.com/paularlott/mcp/openai"
	"github.com/paularlott/scriptling/object"
)

func TestSetToolsMethod(t *testing.T) {
	// Create a client
	client, err := openai.New(openai.Config{
		BaseURL: "http://localhost:1234/v1",
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Wrap it
	instance := createClientInstance(client)

	// Create tools list
	tools := []any{
		map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        "test_tool",
				"description": "A test tool",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"arg1": map[string]any{"type": "string"},
					},
					"required": []any{"arg1"},
				},
			},
		},
	}

	// Call set_tools
	result := setToolsMethod(instance, context.Background(), tools)

	// Check result
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("Expected Null, got %T: %v", result, result)
	}

	// Verify tools were set
	customTools := client.GetCustomTools()
	if len(customTools) != 1 {
		t.Errorf("Expected 1 custom tool, got %d", len(customTools))
	}

	if len(customTools) > 0 {
		if customTools[0].Function.Name != "test_tool" {
			t.Errorf("Expected tool name 'test_tool', got '%s'", customTools[0].Function.Name)
		}
	}
}

func TestSetToolsMethodErrors(t *testing.T) {
	client, _ := openai.New(openai.Config{BaseURL: "http://localhost:1234/v1"})
	instance := createClientInstance(client)

	tests := []struct {
		name  string
		tools []any
		want  string
	}{
		{
			name:  "non-dict tool",
			tools: []any{"not a dict"},
			want:  "set_tools: each tool must be a dict",
		},
		{
			name: "missing function",
			tools: []any{
				map[string]any{"type": "function"},
			},
			want: "set_tools: each tool must have a 'function' dict",
		},
		{
			name: "missing name",
			tools: []any{
				map[string]any{
					"type": "function",
					"function": map[string]any{
						"description": "test",
					},
				},
			},
			want: "set_tools: function name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setToolsMethod(instance, context.Background(), tt.tools)
			if err, ok := result.(*object.Error); ok {
				if err.Message != tt.want {
					t.Errorf("Expected error '%s', got '%s'", tt.want, err.Message)
				}
			} else {
				t.Errorf("Expected error, got %T", result)
			}
		})
	}
}
