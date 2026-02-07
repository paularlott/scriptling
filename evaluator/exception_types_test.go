package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func TestExceptionTypeMatching(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		isError  bool
	}{
		{
			name: "bare except catches all",
			input: `
try:
    x = 1 / 0
except:
    result = "caught"
result
`,
			expected: "caught",
		},
		{
			name: "except Exception catches all",
			input: `
try:
    x = 1 / 0
except Exception as e:
    result = "caught"
result
`,
			expected: "caught",
		},
		{
			name: "specific exception type doesn't match",
			input: `
result = "not caught"
try:
    try:
        x = 1 / 0
    except ValueError as e:
        result = "caught ValueError"
except:
    result = "outer caught"
result
`,
			expected: "outer caught",
		},
		{
			name: "raise Exception with message",
			input: `
try:
    raise Exception("test error")
except Exception as e:
    result = str(e)
result
`,
			expected: "test error",
		},
		{
			name: "raise ValueError",
			input: `
try:
    raise ValueError("bad value")
except ValueError as e:
    result = str(e)
result
`,
			expected: "bad value",
		},
		{
			name: "ValueError doesn't match TypeError",
			input: `
try:
    raise ValueError("bad value")
except TypeError as e:
    result = "caught TypeError"
except:
    result = "caught by bare except"
result
`,
			expected: "caught by bare except",
		},
		{
			name: "Exception catches ValueError",
			input: `
try:
    raise ValueError("bad value")
except Exception as e:
    result = "caught"
result
`,
			expected: "caught",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}

			env := object.NewEnvironment()
			result := EvalWithContext(context.Background(), program, env)

			if tt.isError {
				if !object.IsError(result) {
					t.Fatalf("expected error, got %T (%+v)", result, result)
				}
				return
			}

			if object.IsError(result) {
				t.Fatalf("unexpected error: %s", result.Inspect())
			}

			if result.Inspect() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.Inspect())
			}
		})
	}
}

func TestExceptionInspect(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		contains string
	}{
		{
			name: "exception variable contains message",
			input: `
try:
    raise Exception("test message")
except Exception as e:
    result = str(e)
result
`,
			contains: "test message",
		},
		{
			name: "error converted to exception",
			input: `
try:
    x = 1 / 0
except Exception as e:
    result = str(e)
result
`,
			contains: "division by zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := lexer.New(tt.input)
			p := parser.New(l)
			program := p.ParseProgram()

			if len(p.Errors()) != 0 {
				t.Fatalf("parser errors: %v", p.Errors())
			}

			env := object.NewEnvironment()
			result := EvalWithContext(context.Background(), program, env)

			if object.IsError(result) {
				t.Fatalf("unexpected error: %s", result.Inspect())
			}

			resultStr := result.Inspect()
			if len(resultStr) < len(tt.contains) || resultStr[:len(tt.contains)] != tt.contains && 
				!contains(resultStr, tt.contains) {
				t.Errorf("expected result to contain %q, got %q", tt.contains, resultStr)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && 
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || 
		findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
