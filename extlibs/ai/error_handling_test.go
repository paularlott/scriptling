package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/mcp/ai"
	"github.com/paularlott/scriptling/object"
)

func TestNewClientErrors(t *testing.T) {
	tests := []struct {
		name    string
		service string
		wantErr string
	}{
		{
			name:    "unsupported service",
			service: "unsupported",
			wantErr: "unsupported provider: unsupported",
		},
		{
			name:    "invalid service",
			service: "invalid_provider",
			wantErr: "unsupported provider: invalid_provider",
		},
	}

	lib := buildLibrary()
	newClientFunc := lib.Functions()["new_client"]

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kwargs := object.NewKwargs(map[string]object.Object{
				"provider": &object.String{Value: tt.service},
			})

			result := newClientFunc.Fn(context.Background(), kwargs, &object.String{Value: ""})

			if tt.wantErr == "" {
				if errObj, ok := result.(*object.Error); ok {
					t.Errorf("Expected success, got error: %v", errObj.Message)
				}
				if result == nil {
					t.Error("Expected client instance, got nil")
				}
			} else {
				errObj, ok := result.(*object.Error)
				if !ok {
					t.Errorf("Expected error containing %q, got success", tt.wantErr)
				} else if !strings.Contains(errObj.Message, tt.wantErr) {
					t.Errorf("Expected error containing %q, got %q", tt.wantErr, errObj.Message)
				}
			}
		})
	}
}

func TestProviderConstants(t *testing.T) {
	lib := buildLibrary()
	constants := lib.Constants()

	tests := []struct {
		name     string
		expected string
	}{
		{"OPENAI", string(ai.ProviderOpenAI)},
		{"CLAUDE", string(ai.ProviderClaude)},
		{"GEMINI", string(ai.ProviderGemini)},
		{"OLLAMA", string(ai.ProviderOllama)},
		{"ZAI", string(ai.ProviderZAi)},
		{"MISTRAL", string(ai.ProviderMistral)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constant, ok := constants[tt.name]
			if !ok {
				t.Errorf("Constant %s not found", tt.name)
				return
			}

			str, ok := constant.(*object.String)
			if !ok {
				t.Errorf("Constant %s is not a string, got %T", tt.name, constant)
				return
			}

			if str.Value != tt.expected {
				t.Errorf("Constant %s = %q, want %q", tt.name, str.Value, tt.expected)
			}
		})
	}
}
