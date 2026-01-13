package ai

import (
	"testing"
)

func TestAILibraryConstants(t *testing.T) {
	if AILibraryName != "ai" {
		t.Errorf("AILibraryName = %q, want %q", AILibraryName, "ai")
	}

	if AILibraryDesc == "" {
		t.Error("AILibraryDesc should not be empty")
	}
}

func TestGetOpenAIClientClass(t *testing.T) {
	class := GetOpenAIClientClass()

	if class == nil {
		t.Error("GetOpenAIClientClass() returned nil")
	}

	if class.Name != "OpenAIClient" {
		t.Errorf("Class name = %q, want %q", class.Name, "OpenAIClient")
	}
}

func TestGetChatStreamClass(t *testing.T) {
	class := GetChatStreamClass()

	if class == nil {
		t.Error("GetChatStreamClass() returned nil")
	}

	if class.Name != "ChatStream" {
		t.Errorf("Class name = %q, want %q", class.Name, "ChatStream")
	}
}

func TestGetOpenAIClientClassSingleton(t *testing.T) {
	class1 := GetOpenAIClientClass()
	class2 := GetOpenAIClientClass()

	if class1 != class2 {
		t.Error("GetOpenAIClientClass() should return the same instance (singleton)")
	}
}

func TestGetChatStreamClassSingleton(t *testing.T) {
	class1 := GetChatStreamClass()
	class2 := GetChatStreamClass()

	if class1 != class2 {
		t.Error("GetChatStreamClass() should return the same instance (singleton)")
	}
}

func TestConvertMapsToOpenAI(t *testing.T) {
	tests := []struct {
		name     string
		messages []map[string]any
		wantLen  int
	}{
		{
			name: "single message with role and content",
			messages: []map[string]any{
				{"role": "user", "content": "Hello"},
			},
			wantLen: 1,
		},
		{
			name: "multiple messages",
			messages: []map[string]any{
				{"role": "system", "content": "You are helpful"},
				{"role": "user", "content": "Hello"},
				{"role": "assistant", "content": "Hi there!"},
			},
			wantLen: 3,
		},
		{
			name: "message with tool_call_id",
			messages: []map[string]any{
				{"role": "tool", "content": "result", "tool_call_id": "call_123"},
			},
			wantLen: 1,
		},
		{
			name:     "empty messages",
			messages: []map[string]any{},
			wantLen:  0,
		},
		{
			name: "message with only role",
			messages: []map[string]any{
				{"role": "user"},
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMapsToOpenAI(tt.messages)
			if len(result) != tt.wantLen {
				t.Errorf("convertMapsToOpenAI() length = %d, want %d", len(result), tt.wantLen)
			}
		})
	}
}

func TestConvertMapsToOpenAIValues(t *testing.T) {
	messages := []map[string]any{
		{"role": "user", "content": "Hello", "tool_call_id": "call_123"},
	}

	result := convertMapsToOpenAI(messages)

	if len(result) != 1 {
		t.Fatalf("convertMapsToOpenAI() length = %d, want 1", len(result))
	}

	msg := result[0]
	if msg.Role != "user" {
		t.Errorf("msg.Role = %q, want %q", msg.Role, "user")
	}

	if msg.Content != "Hello" {
		t.Errorf("msg.Content = %v, want %q", msg.Content, "Hello")
	}

	if msg.ToolCallID != "call_123" {
		t.Errorf("msg.ToolCallID = %q, want %q", msg.ToolCallID, "call_123")
	}
}
