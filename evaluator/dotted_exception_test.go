package evaluator

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

func TestDottedExceptionTypeMatching(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "except requests.HTTPError catches HTTPError exception",
			input: `
import requests
try:
    raise HTTPError("not found")
except requests.HTTPError as e:
    result = "caught"
result
`,
			expected: "caught",
		},
		{
			name: "except requests.HTTPError does not catch ValueError",
			input: `
import requests
try:
    try:
        raise ValueError("bad")
    except requests.HTTPError:
        result = "wrong"
except:
    result = "outer"
result
`,
			expected: "outer",
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
			// Register a minimal requests namespace with HTTPError constant
			requestsDict := object.NewStringDict(map[string]object.Object{
				"HTTPError": &object.String{Value: "HTTPError"},
			})
			env.Set("requests", requestsDict)
			// Register HTTPError as a callable that raises an exception
			env.Set("HTTPError", &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					msg := "HTTPError"
					if len(args) > 0 {
						if s, err := args[0].AsString(); err == nil {
							msg = s
						}
					}
					return &object.Exception{ExceptionType: "HTTPError", Message: msg}
				},
			})
			env.Set("ValueError", &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					msg := "ValueError"
					if len(args) > 0 {
						if s, err := args[0].AsString(); err == nil {
							msg = s
						}
					}
					return &object.Exception{ExceptionType: "ValueError", Message: msg}
				},
			})
			// import callback that does nothing (requests already in env)
			env.SetImportCallback(func(name string) error { return nil })

			result := EvalWithContext(context.Background(), program, env)

			if object.IsError(result) {
				t.Fatalf("unexpected error: %s", result.Inspect())
			}
			if result.Inspect() != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result.Inspect())
			}
		})
	}
}
