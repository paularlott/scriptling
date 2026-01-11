package scriptling

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestBuilderPositionalArgs tests the builder with positional arguments only.
func TestBuilderPositionalArgs(t *testing.T) {
	builder := object.NewLibraryBuilder("test", "Test library")

	// Function with positional arguments only
	builder.Function("add", func(a, b int) int {
		return a + b
	})

	lib := builder.Build()

	// Test the function
	functions := lib.Functions()
	fn, ok := functions["add"]
	if !ok {
		t.Fatal("add function not found")
	}

	result := fn.Fn(context.Background(), object.NewKwargs(map[string]object.Object{}),
		object.NewInteger(3), object.NewInteger(5))

	if intResult, ok := result.(*object.Integer); !ok {
		t.Fatalf("expected Integer, got %T", result)
	} else if intResult.Value != 8 {
		t.Errorf("expected 8, got %d", intResult.Value)
	}
}

// TestBuilderContextOnly tests functions with context parameter only.
func TestBuilderContextOnly(t *testing.T) {
	builder := object.NewLibraryBuilder("test", "Test library")

	// Function with context only
	builder.Function("get_context", func(ctx context.Context) string {
		if ctx == nil {
			return "no context"
		}
		return "has context"
	})

	lib := builder.Build()
	functions := lib.Functions()
	fn, ok := functions["get_context"]
	if !ok {
		t.Fatal("get_context function not found")
	}

	result := fn.Fn(context.Background(), object.NewKwargs(map[string]object.Object{}))

	if strResult, ok := result.(*object.String); !ok {
		t.Fatalf("expected String, got %T", result)
	} else if strResult.Value != "has context" {
		t.Errorf("expected 'has context', got %s", strResult.Value)
	}
}

// TestBuilderKwargsOnly tests the builder with kwargs only.
func TestBuilderKwargsOnly(t *testing.T) {
	builder := object.NewLibraryBuilder("test", "Test library")

	// Function with only kwargs (no positional args except Kwargs)
	builder.Function("connect", func(kwargs object.Kwargs) (string, error) {
		host, objErr := kwargs.GetString("host", "localhost")
		if objErr != nil {
			return "", fmt.Errorf("failed to get host: %v", objErr)
		}
		port, objErr := kwargs.GetInt("port", 8080)
		if objErr != nil {
			return "", fmt.Errorf("failed to get port: %v", objErr)
		}
		return fmt.Sprintf("%s:%d", host, port), nil
	})

	lib := builder.Build()

	// Test with kwargs
	tests := []struct {
		name     string
		kwargs   map[string]object.Object
		expected string
	}{
		{
			name:     "default values",
			kwargs:   map[string]object.Object{},
			expected: "localhost:8080",
		},
		{
			name: "custom host",
			kwargs: map[string]object.Object{
				"host": &object.String{Value: "example.com"},
			},
			expected: "example.com:8080",
		},
		{
			name: "custom port",
			kwargs: map[string]object.Object{
				"port": object.NewInteger(9000),
			},
			expected: "localhost:9000",
		},
		{
			name: "custom host and port",
			kwargs: map[string]object.Object{
				"host": &object.String{Value: "example.com"},
				"port": object.NewInteger(443),
			},
			expected: "example.com:443",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			functions := lib.Functions()
			fn, ok := functions["connect"]
			if !ok {
				t.Fatal("connect function not found")
			}

			result := fn.Fn(context.Background(), object.NewKwargs(tt.kwargs))

			if strResult, ok := result.(*object.String); !ok {
				t.Fatalf("expected String, got %T", result)
			} else if strResult.Value != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, strResult.Value)
			}
		})
	}
}

// TestBuilderContextKwargs tests context + kwargs parameters.
func TestBuilderContextKwargs(t *testing.T) {
	builder := object.NewLibraryBuilder("test", "Test library")

	// Function with context and kwargs
	builder.Function("timeout_connect", func(ctx context.Context, kwargs object.Kwargs) (string, error) {
		host, objErr := kwargs.GetString("host", "localhost")
		if objErr != nil {
			return "", fmt.Errorf("failed to get host: %v", objErr)
		}
		port, objErr := kwargs.GetInt("port", 8080)
		if objErr != nil {
			return "", fmt.Errorf("failed to get port: %v", objErr)
		}

		// Check context
		if ctx == nil {
			return "", fmt.Errorf("no context")
		}

		return fmt.Sprintf("%s:%d", host, port), nil
	})

	lib := builder.Build()
	functions := lib.Functions()
	fn, ok := functions["timeout_connect"]
	if !ok {
		t.Fatal("timeout_connect function not found")
	}

	result := fn.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"host": &object.String{Value: "test.com"},
		"port": object.NewInteger(443),
	}))

	if strResult, ok := result.(*object.String); !ok {
		t.Fatalf("expected String, got %T", result)
	} else if strResult.Value != "test.com:443" {
		t.Errorf("expected 'test.com:443', got %s", strResult.Value)
	}
}

// TestBuilderContextPositional tests context + positional parameters.
func TestBuilderContextPositional(t *testing.T) {
	builder := object.NewLibraryBuilder("test", "Test library")

	// Function with context and positional args
	builder.Function("ctx_add", func(ctx context.Context, a, b int) int {
		if ctx == nil {
			return -1
		}
		return a + b
	})

	lib := builder.Build()
	functions := lib.Functions()
	fn, ok := functions["ctx_add"]
	if !ok {
		t.Fatal("ctx_add function not found")
	}

	result := fn.Fn(context.Background(), object.NewKwargs(map[string]object.Object{}),
		object.NewInteger(10), object.NewInteger(20))

	if intResult, ok := result.(*object.Integer); !ok {
		t.Fatalf("expected Integer, got %T", result)
	} else if intResult.Value != 30 {
		t.Errorf("expected 30, got %d", intResult.Value)
	}
}

// TestBuilderMixedContextKwargsPositional tests context + kwargs + positional.
func TestBuilderMixedContextKwargsPositional(t *testing.T) {
	builder := object.NewLibraryBuilder("test", "Test library")

	// Function with context, kwargs, and positional args
	builder.Function("format", func(ctx context.Context, kwargs object.Kwargs, name string, count int) (string, error) {
		if ctx == nil {
			return "", fmt.Errorf("no context")
		}

		prefix, objErr := kwargs.GetString("prefix", ">")
		if objErr != nil {
			return "", fmt.Errorf("failed to get prefix: %v", objErr)
		}
		suffix, objErr := kwargs.GetString("suffix", "<")
		if objErr != nil {
			return "", fmt.Errorf("failed to get suffix: %v", objErr)
		}
		return fmt.Sprintf("%s %s: %d times %s", prefix, name, count, suffix), nil
	})

	lib := builder.Build()

	// Test with positional args and kwargs
	tests := []struct {
		name     string
		args     []object.Object
		kwargs   map[string]object.Object
		expected string
	}{
		{
			name:     "defaults only",
			args:     []object.Object{&object.String{Value: "task"}, object.NewInteger(5)},
			kwargs:   map[string]object.Object{},
			expected: "> task: 5 times <",
		},
		{
			name:     "custom prefix",
			args:     []object.Object{&object.String{Value: "task"}, object.NewInteger(3)},
			kwargs: map[string]object.Object{
				"prefix": &object.String{Value: ">>>"},
			},
			expected: ">>> task: 3 times <",
		},
		{
			name:     "custom suffix",
			args:     []object.Object{&object.String{Value: "task"}, object.NewInteger(10)},
			kwargs: map[string]object.Object{
				"suffix": &object.String{Value: "<<<"},
			},
			expected: "> task: 10 times <<<",
		},
		{
			name:     "custom prefix and suffix",
			args:     []object.Object{&object.String{Value: "task"}, object.NewInteger(7)},
			kwargs: map[string]object.Object{
				"prefix": &object.String{Value: "["},
				"suffix": &object.String{Value: "]"},
			},
			expected: "[ task: 7 times ]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			functions := lib.Functions()
			fn, ok := functions["format"]
			if !ok {
				t.Fatal("format function not found")
			}

			result := fn.Fn(context.Background(), object.NewKwargs(tt.kwargs), tt.args...)

			if strResult, ok := result.(*object.String); !ok {
				t.Fatalf("expected String, got %T", result)
			} else if strResult.Value != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, strResult.Value)
			}
		})
	}
}

// TestBuilderKwargsWithAllTypes tests kwargs with all supported types.
func TestBuilderKwargsWithAllTypes(t *testing.T) {
	builder := object.NewLibraryBuilder("test", "Test library")

	builder.Function("all_types", func(kwargs object.Kwargs) (string, error) {
		s, objErr := kwargs.GetString("str", "default")
		if objErr != nil {
			return "", fmt.Errorf("failed to get str: %v", objErr)
		}
		i, objErr := kwargs.GetInt("int", 42)
		if objErr != nil {
			return "", fmt.Errorf("failed to get int: %v", objErr)
		}
		f, objErr := kwargs.GetFloat("float", 3.14)
		if objErr != nil {
			return "", fmt.Errorf("failed to get float: %v", objErr)
		}
		b, objErr := kwargs.GetBool("bool", true)
		if objErr != nil {
			return "", fmt.Errorf("failed to get bool: %v", objErr)
		}
		return fmt.Sprintf("str=%s int=%d float=%.2f bool=%t", s, i, f, b), nil
	})

	lib := builder.Build()

	// Test with all types
	functions := lib.Functions()
	fn, ok := functions["all_types"]
	if !ok {
		t.Fatal("all_types function not found")
	}

	result := fn.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"str":   &object.String{Value: "hello"},
		"int":   object.NewInteger(100),
		"float": &object.Float{Value: 2.718},
		"bool":  &object.Boolean{Value: false},
	}))

	if strResult, ok := result.(*object.String); !ok {
		t.Fatalf("expected String, got %T", result)
	} else if strResult.Value != "str=hello int=100 float=2.72 bool=false" {
		t.Errorf("unexpected result: %s", strResult.Value)
	}
}

// TestBuilderKwargsMustHelpers tests the Must* helper methods.
func TestBuilderKwargsMustHelpers(t *testing.T) {
	builder := object.NewLibraryBuilder("test", "Test library")

	builder.Function("must_test", func(kwargs object.Kwargs) string {
		// Must helpers should return defaults without error checking
		s := kwargs.MustGetString("str", "default")
		i := kwargs.MustGetInt("int", 42)
		return fmt.Sprintf("%s:%d", s, i)
	})

	lib := builder.Build()

	// Test with valid kwargs
	functions := lib.Functions()
	fn, ok := functions["must_test"]
	if !ok {
		t.Fatal("must_test function not found")
	}

	result := fn.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"str": &object.String{Value: "hello"},
	}))

	if strResult, ok := result.(*object.String); !ok {
		t.Fatalf("expected String, got %T", result)
	} else if strResult.Value != "hello:42" {
		t.Errorf("expected 'hello:42', got %s", strResult.Value)
	}
}

// TestBuilderKwargsHasLenKeys tests Kwargs helper methods.
func TestBuilderKwargsHasLenKeys(t *testing.T) {
	builder := object.NewLibraryBuilder("test", "Test library")

	builder.Function("helpers", func(kwargs object.Kwargs) string {
		result := fmt.Sprintf("len=%d", kwargs.Len())
		if kwargs.Has("a") {
			result += " has_a=true"
		}
		if kwargs.Has("b") {
			result += " has_b=true"
		}
		keys := kwargs.Keys()
		result += fmt.Sprintf(" keys=%v", keys)
		return result
	})

	lib := builder.Build()

	functions := lib.Functions()
	fn, ok := functions["helpers"]
	if !ok {
		t.Fatal("helpers function not found")
	}

	result := fn.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"a": object.NewInteger(1),
		"c": &object.String{Value: "test"},
	}))

	if strResult, ok := result.(*object.String); !ok {
		t.Fatalf("expected String, got %T", result)
	} else {
		// Check that result contains expected parts (keys order is non-deterministic)
		got := strResult.Value
		// Should contain len=2, has_a=true, and keys=[a c] (order may vary)
		if !strings.Contains(got, "len=2") {
			t.Errorf("expected len=2, got %s", got)
		}
		if !strings.Contains(got, "has_a=true") {
			t.Errorf("expected has_a=true, got %s", got)
		}
		if strings.Contains(got, "has_b=true") {
			t.Errorf("unexpected has_b=true, got %s", got)
		}
		if !strings.Contains(got, "keys=") {
			t.Errorf("expected keys=, got %s", got)
		}
	}
}