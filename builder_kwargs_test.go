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
	} else if intResult.IntValue() != 8 {
		t.Errorf("expected 8, got %d", intResult.IntValue())
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
	} else if strResult.StringValue() != "has context" {
		t.Errorf("expected 'has context', got %s", strResult.StringValue())
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
				"host": object.NewString("example.com"),
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
				"host": object.NewString("example.com"),
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
			} else if strResult.StringValue() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, strResult.StringValue())
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
		"host": object.NewString("test.com"),
		"port": object.NewInteger(443),
	}))

	if strResult, ok := result.(*object.String); !ok {
		t.Fatalf("expected String, got %T", result)
	} else if strResult.StringValue() != "test.com:443" {
		t.Errorf("expected 'test.com:443', got %s", strResult.StringValue())
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
	} else if intResult.IntValue() != 30 {
		t.Errorf("expected 30, got %d", intResult.IntValue())
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
			args:     []object.Object{object.NewString("task"), object.NewInteger(5)},
			kwargs:   map[string]object.Object{},
			expected: "> task: 5 times <",
		},
		{
			name: "custom prefix",
			args: []object.Object{object.NewString("task"), object.NewInteger(3)},
			kwargs: map[string]object.Object{
				"prefix": object.NewString(">>>"),
			},
			expected: ">>> task: 3 times <",
		},
		{
			name: "custom suffix",
			args: []object.Object{object.NewString("task"), object.NewInteger(10)},
			kwargs: map[string]object.Object{
				"suffix": object.NewString("<<<"),
			},
			expected: "> task: 10 times <<<",
		},
		{
			name: "custom prefix and suffix",
			args: []object.Object{object.NewString("task"), object.NewInteger(7)},
			kwargs: map[string]object.Object{
				"prefix": object.NewString("["),
				"suffix": object.NewString("]"),
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
			} else if strResult.StringValue() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, strResult.StringValue())
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
		"str":   object.NewString("hello"),
		"int":   object.NewInteger(100),
		"float": object.NewFloat(2.718),
		"bool":  object.NewBoolean(false),
	}))

	if strResult, ok := result.(*object.String); !ok {
		t.Fatalf("expected String, got %T", result)
	} else if strResult.StringValue() != "str=hello int=100 float=2.72 bool=false" {
		t.Errorf("unexpected result: %s", strResult.StringValue())
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
		"str": object.NewString("hello"),
	}))

	if strResult, ok := result.(*object.String); !ok {
		t.Fatalf("expected String, got %T", result)
	} else if strResult.StringValue() != "hello:42" {
		t.Errorf("expected 'hello:42', got %s", strResult.StringValue())
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
		"c": object.NewString("test"),
	}))

	if strResult, ok := result.(*object.String); !ok {
		t.Fatalf("expected String, got %T", result)
	} else {
		// Check that result contains expected parts (keys order is non-deterministic)
		got := strResult.StringValue()
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

func TestFunctionBuilderSimple(t *testing.T) {
	fb := object.NewFunctionBuilder()
	fb.Function(func(a, b int) int { return a + b })
	fn := fb.Build()

	result := fn(context.Background(), object.NewKwargs(nil), object.NewInteger(3), object.NewInteger(4))
	if intResult, ok := result.(*object.Integer); !ok || intResult.IntValue() != 7 {
		t.Errorf("expected 7, got %v", result)
	}
}

func TestFunctionBuilderWithHelp(t *testing.T) {
	fb := object.NewFunctionBuilder()
	fb.FunctionWithHelp(func(x float64) float64 { return x * 2 }, "double(x) - doubles the value")
	fn := fb.Build()

	// Test the function works
	result := fn(context.Background(), object.NewKwargs(nil), object.NewFloat(3.5))
	if floatResult, ok := result.(*object.Float); !ok || floatResult.FloatValue() != 7.0 {
		t.Errorf("expected 7.0, got %v", result)
	}
}

func TestFunctionBuilderContext(t *testing.T) {
	fb := object.NewFunctionBuilder()
	fb.Function(func(ctx context.Context, a int) string {
		return fmt.Sprintf("got %d", a)
	})
	fn := fb.Build()

	result := fn(context.Background(), object.NewKwargs(nil), object.NewInteger(42))
	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "got 42" {
		t.Errorf("expected 'got 42', got %v", result)
	}
}

func TestFunctionBuilderKwargs(t *testing.T) {
	fb := object.NewFunctionBuilder()
	fb.Function(func(kwargs object.Kwargs) (string, error) {
		host, _ := kwargs.GetString("host", "localhost")
		port, _ := kwargs.GetInt("port", 8080)
		return fmt.Sprintf("%s:%d", host, port), nil
	})
	fn := fb.Build()

	kwargs := object.NewKwargs(map[string]object.Object{
		"host": object.NewString("example.com"),
		"port": object.NewInteger(9000),
	})
	result := fn(context.Background(), kwargs)
	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "example.com:9000" {
		t.Errorf("expected 'example.com:9000', got %v", result)
	}
}

func TestFunctionBuilderMixed(t *testing.T) {
	fb := object.NewFunctionBuilder()
	fb.Function(func(ctx context.Context, kwargs object.Kwargs, name string) string {
		prefix, _ := kwargs.GetString("prefix", "Hello")
		return fmt.Sprintf("%s, %s!", prefix, name)
	})
	fn := fb.Build()

	kwargs := object.NewKwargs(map[string]object.Object{
		"prefix": object.NewString("Hi"),
	})
	result := fn(context.Background(), kwargs, object.NewString("World"))
	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "Hi, World!" {
		t.Errorf("expected 'Hi, World!', got %v", result)
	}
}

func TestFunctionBuilderErrorReturn(t *testing.T) {
	fb := object.NewFunctionBuilder()
	fb.Function(func(a, b int) (int, error) {
		if b == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return a / b, nil
	})
	fn := fb.Build()

	// Test success
	result := fn(context.Background(), object.NewKwargs(nil), object.NewInteger(10), object.NewInteger(2))
	if intResult, ok := result.(*object.Integer); !ok || intResult.IntValue() != 5 {
		t.Errorf("expected 5, got %v", result)
	}

	// Test error
	result = fn(context.Background(), object.NewKwargs(nil), object.NewInteger(10), object.NewInteger(0))
	if errResult, ok := result.(*object.Error); !ok || !strings.Contains(errResult.Message, "division by zero") {
		t.Errorf("expected error containing 'division by zero', got %v", result)
	}
}

func TestFunctionBuilderNoFunction(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when building without function")
		}
	}()
	fb := object.NewFunctionBuilder()
	fb.Build() // Should panic
}

func TestFunctionBuilderMultipleFunctions(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when registering multiple functions")
		}
	}()
	fb := object.NewFunctionBuilder()
	fb.Function(func() {})
	fb.Function(func() {}) // Should panic
}

func TestClassBuilderSimple(t *testing.T) {
	cb := object.NewClassBuilder("TestClass")
	cb.Method("greet", func(self *object.Instance, name string) string {
		return "Hello, " + name
	})
	class := cb.Build()

	if class.Name != "TestClass" {
		t.Errorf("expected class name 'TestClass', got %s", class.Name)
	}

	if len(class.Methods) != 1 {
		t.Errorf("expected 1 method, got %d", len(class.Methods))
	}

	method, ok := class.Methods["greet"]
	if !ok {
		t.Fatal("greet method not found")
	}

	builtin, ok := method.(*object.Builtin)
	if !ok {
		t.Fatal("method is not a Builtin")
	}

	// Create a test instance
	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	// Call the method
	result := builtin.Fn(context.Background(), object.NewKwargs(nil), instance, object.NewString("World"))
	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "Hello, World" {
		t.Errorf("expected 'Hello, World', got %v", result)
	}
}

func TestClassBuilderWithHelp(t *testing.T) {
	cb := object.NewClassBuilder("Person")
	cb.MethodWithHelp("introduce", func(self *object.Instance) string {
		return "I am a person"
	}, "Return an introduction")
	class := cb.Build()

	method := class.Methods["introduce"].(*object.Builtin)
	if method.HelpText != "Return an introduction" {
		t.Errorf("expected help text 'Return an introduction', got %s", method.HelpText)
	}
}

func TestClassBuilderMultipleMethods(t *testing.T) {
	cb := object.NewClassBuilder("Calculator")
	cb.Method("add", func(self *object.Instance, a, b int) int {
		return a + b
	})
	cb.Method("multiply", func(self *object.Instance, a, b int) int {
		return a * b
	})
	class := cb.Build()

	if len(class.Methods) != 2 {
		t.Errorf("expected 2 methods, got %d", len(class.Methods))
	}

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	// Test add method
	addMethod := class.Methods["add"].(*object.Builtin)
	result := addMethod.Fn(context.Background(), object.NewKwargs(nil), instance, object.NewInteger(3), object.NewInteger(4))
	if intResult, ok := result.(*object.Integer); !ok || intResult.IntValue() != 7 {
		t.Errorf("expected 7, got %v", result)
	}

	// Test multiply method
	multiplyMethod := class.Methods["multiply"].(*object.Builtin)
	result = multiplyMethod.Fn(context.Background(), object.NewKwargs(nil), instance, object.NewInteger(5), object.NewInteger(6))
	if intResult, ok := result.(*object.Integer); !ok || intResult.IntValue() != 30 {
		t.Errorf("expected 30, got %v", result)
	}
}

func TestClassBuilderWithBaseClass(t *testing.T) {
	baseClass := &object.Class{
		Name:    "Base",
		Methods: map[string]object.Object{},
	}

	cb := object.NewClassBuilder("Derived")
	cb.BaseClass(baseClass)
	cb.Method("special", func(self *object.Instance) string {
		return "special method"
	})
	class := cb.Build()

	if class.BaseClass != baseClass {
		t.Error("base class not set correctly")
	}

	if class.Name != "Derived" {
		t.Errorf("expected name 'Derived', got %s", class.Name)
	}
}

func TestClassBuilderMethodWithError(t *testing.T) {
	cb := object.NewClassBuilder("TestClass")
	cb.Method("divide", func(self *object.Instance, a, b int) (int, error) {
		if b == 0 {
			return 0, fmt.Errorf("division by zero")
		}
		return a / b, nil
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["divide"].(*object.Builtin)

	// Test success
	result := method.Fn(context.Background(), object.NewKwargs(nil), instance, object.NewInteger(10), object.NewInteger(2))
	if intResult, ok := result.(*object.Integer); !ok || intResult.IntValue() != 5 {
		t.Errorf("expected 5, got %v", result)
	}

	// Test error
	result = method.Fn(context.Background(), object.NewKwargs(nil), instance, object.NewInteger(10), object.NewInteger(0))
	if errResult, ok := result.(*object.Error); !ok || !strings.Contains(errResult.Message, "division by zero") {
		t.Errorf("expected error containing 'division by zero', got %v", result)
	}
}

// TestClassBuilderKwargsOnly tests class methods with only kwargs parameter.
func TestClassBuilderKwargsOnly(t *testing.T) {
	cb := object.NewClassBuilder("Connector")
	cb.Method("connect", func(self *object.Instance, kwargs object.Kwargs) (string, error) {
		host, _ := kwargs.GetString("host", "localhost")
		port, _ := kwargs.GetInt("port", 8080)
		return fmt.Sprintf("%s:%d", host, port), nil
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["connect"].(*object.Builtin)

	// Test with defaults
	result := method.Fn(context.Background(), object.NewKwargs(map[string]object.Object{}), instance)
	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "localhost:8080" {
		t.Errorf("expected 'localhost:8080', got %v", result)
	}

	// Test with custom values
	result = method.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"host": object.NewString("example.com"),
		"port": object.NewInteger(443),
	}), instance)
	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "example.com:443" {
		t.Errorf("expected 'example.com:443', got %v", result)
	}
}

// TestClassBuilderContextKwargs tests class methods with context and kwargs.
func TestClassBuilderContextKwargs(t *testing.T) {
	cb := object.NewClassBuilder("ContextConnector")
	cb.Method("connect", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs) (string, error) {
		if ctx == nil {
			return "", fmt.Errorf("no context")
		}
		host, _ := kwargs.GetString("host", "localhost")
		port, _ := kwargs.GetInt("port", 8080)
		return fmt.Sprintf("%s:%d", host, port), nil
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["connect"].(*object.Builtin)

	result := method.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"host": object.NewString("test.com"),
	}), instance)

	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "test.com:8080" {
		t.Errorf("expected 'test.com:8080', got %v", result)
	}
}

// TestClassBuilderMixedKwargsPositional tests class methods with kwargs and positional args.
func TestClassBuilderMixedKwargsPositional(t *testing.T) {
	cb := object.NewClassBuilder("Formatter")
	cb.Method("format", func(self *object.Instance, kwargs object.Kwargs, name string, count int) (string, error) {
		prefix, _ := kwargs.GetString("prefix", ">")
		suffix, _ := kwargs.GetString("suffix", "<")
		return fmt.Sprintf("%s %s: %d times %s", prefix, name, count, suffix), nil
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["format"].(*object.Builtin)

	// Test with defaults
	result := method.Fn(context.Background(), object.NewKwargs(map[string]object.Object{}), instance,
		object.NewString("task"), object.NewInteger(5))
	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "> task: 5 times <" {
		t.Errorf("expected '> task: 5 times <', got %v", result)
	}

	// Test with custom kwargs
	result = method.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"prefix": object.NewString("["),
		"suffix": object.NewString("]"),
	}), instance, object.NewString("job"), object.NewInteger(10))
	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "[ job: 10 times ]" {
		t.Errorf("expected '[ job: 10 times ]', got %v", result)
	}
}

// TestClassBuilderContextKwargsPositional tests class methods with context, kwargs, and positional args.
func TestClassBuilderContextKwargsPositional(t *testing.T) {
	cb := object.NewClassBuilder("ContextFormatter")
	cb.Method("format", func(self *object.Instance, ctx context.Context, kwargs object.Kwargs, name string) (string, error) {
		if ctx == nil {
			return "", fmt.Errorf("no context")
		}
		prefix, _ := kwargs.GetString("prefix", "Hello")
		return fmt.Sprintf("%s, %s!", prefix, name), nil
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["format"].(*object.Builtin)

	result := method.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"prefix": object.NewString("Hi"),
	}), instance, object.NewString("World"))

	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "Hi, World!" {
		t.Errorf("expected 'Hi, World!', got %v", result)
	}
}

// TestClassBuilderMustHelpers tests class methods with Must* helper methods.
func TestClassBuilderMustHelpers(t *testing.T) {
	cb := object.NewClassBuilder("HelperClass")
	cb.Method("get_info", func(self *object.Instance, kwargs object.Kwargs) string {
		name := kwargs.MustGetString("name", "default")
		count := kwargs.MustGetInt("count", 0)
		return fmt.Sprintf("%s:%d", name, count)
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["get_info"].(*object.Builtin)

	result := method.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"name": object.NewString("test"),
	}), instance)

	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "test:0" {
		t.Errorf("expected 'test:0', got %v", result)
	}
}

// TestClassBuilderNoArgs tests class methods with no arguments (except self).
func TestClassBuilderNoArgs(t *testing.T) {
	cb := object.NewClassBuilder("Simple")
	cb.Method("get_value", func(self *object.Instance) int {
		return 42
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["get_value"].(*object.Builtin)

	result := method.Fn(context.Background(), object.NewKwargs(nil), instance)
	if intResult, ok := result.(*object.Integer); !ok || intResult.IntValue() != 42 {
		t.Errorf("expected 42, got %v", result)
	}
}

// TestClassBuilderContextOnly tests class methods with context and no other args (except self).
func TestClassBuilderContextOnly(t *testing.T) {
	cb := object.NewClassBuilder("Contextual")
	cb.Method("check_context", func(self *object.Instance, ctx context.Context) string {
		if ctx != nil {
			return "has context"
		}
		return "no context"
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["check_context"].(*object.Builtin)

	result := method.Fn(context.Background(), object.NewKwargs(nil), instance)
	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "has context" {
		t.Errorf("expected 'has context', got %v", result)
	}
}

// TestClassBuilderAllTypes tests class methods with kwargs containing all supported types.
func TestClassBuilderAllTypes(t *testing.T) {
	cb := object.NewClassBuilder("AllTypes")
	cb.Method("process", func(self *object.Instance, kwargs object.Kwargs) (string, error) {
		s, _ := kwargs.GetString("str", "default")
		i, _ := kwargs.GetInt("int", 42)
		f, _ := kwargs.GetFloat("float", 3.14)
		b, _ := kwargs.GetBool("bool", true)
		return fmt.Sprintf("str=%s int=%d float=%.2f bool=%t", s, i, f, b), nil
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["process"].(*object.Builtin)

	result := method.Fn(context.Background(), object.NewKwargs(map[string]object.Object{
		"str":   object.NewString("hello"),
		"int":   object.NewInteger(100),
		"float": object.NewFloat(2.718),
		"bool":  object.NewBoolean(false),
	}), instance)

	if strResult, ok := result.(*object.String); !ok || strResult.StringValue() != "str=hello int=100 float=2.72 bool=false" {
		t.Errorf("unexpected result: %v", result)
	}
}

// TestClassBuilderVariadic tests class methods with variadic arguments.
func TestClassBuilderVariadic(t *testing.T) {
	cb := object.NewClassBuilder("Variadic")
	cb.Method("sum_all", func(self *object.Instance, nums ...int) int {
		total := 0
		for _, n := range nums {
			total += n
		}
		return total
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["sum_all"].(*object.Builtin)

	result := method.Fn(context.Background(), object.NewKwargs(nil), instance,
		object.NewInteger(1), object.NewInteger(2), object.NewInteger(3), object.NewInteger(4))
	if intResult, ok := result.(*object.Integer); !ok || intResult.IntValue() != 10 {
		t.Errorf("expected 10, got %v", result)
	}
}

// TestClassBuilderContextVariadic tests class methods with context and variadic arguments.
func TestClassBuilderContextVariadic(t *testing.T) {
	cb := object.NewClassBuilder("ContextVariadic")
	cb.Method("sum_with_ctx", func(self *object.Instance, ctx context.Context, nums ...int) int {
		if ctx == nil {
			return -1
		}
		total := 0
		for _, n := range nums {
			total += n
		}
		return total
	})
	class := cb.Build()

	instance := &object.Instance{
		Class:  class,
		Fields: map[string]object.Object{},
	}

	method := class.Methods["sum_with_ctx"].(*object.Builtin)

	result := method.Fn(context.Background(), object.NewKwargs(nil), instance,
		object.NewInteger(5), object.NewInteger(10), object.NewInteger(15))
	if intResult, ok := result.(*object.Integer); !ok || intResult.IntValue() != 30 {
		t.Errorf("expected 30, got %v", result)
	}
}

type trConfig struct {
	values map[string]string
}

type trCounter struct {
	value int64
}

type trGreeter struct {
	greeting string
}

func TestTypedReceiverBasic(t *testing.T) {
	cb := object.NewClassBuilder("Config")
	cb.Constructor(func() *trConfig {
		return &trConfig{values: make(map[string]string)}
	})
	cb.Method("set", func(self *trConfig, key, val string) {
		self.values[key] = val
	})
	cb.Method("get", func(self *trConfig, key string) string {
		return self.values[key]
	})
	class := cb.Build()

	initMethod := class.Methods["__init__"].(*object.Builtin)
	instance := &object.Instance{Class: class, Fields: map[string]object.Object{}}

	result := initMethod.Fn(context.Background(), object.NewKwargs(nil), instance)
	if _, ok := result.(*object.Null); !ok {
		t.Fatalf("expected Null from __init__, got %T", result)
	}

	wrapper, ok := instance.Fields["_receiver"].(*object.ClientWrapper)
	if !ok {
		t.Fatal("expected _receiver field to be a ClientWrapper")
	}
	cfg, ok := wrapper.Client.(*trConfig)
	if !ok {
		t.Fatalf("expected *trConfig, got %T", wrapper.Client)
	}
	if cfg.values == nil {
		t.Fatal("expected values map to be initialized")
	}

	setMethod := class.Methods["set"].(*object.Builtin)
	result = setMethod.Fn(context.Background(), object.NewKwargs(nil), instance,
		object.NewString("host"), object.NewString("localhost"))
	if _, ok := result.(*object.Null); !ok {
		t.Fatalf("expected Null from set, got %T", result)
	}

	getMethod := class.Methods["get"].(*object.Builtin)
	result = getMethod.Fn(context.Background(), object.NewKwargs(nil), instance,
		object.NewString("host"))
	if str, ok := result.(*object.String); !ok || str.StringValue() != "localhost" {
		t.Errorf("expected 'localhost', got %v", result)
	}
}

func TestTypedReceiverConstructorArgs(t *testing.T) {
	cb := object.NewClassBuilder("Counter")
	cb.Constructor(func(start int) *trCounter {
		return &trCounter{value: int64(start)}
	})
	cb.Method("inc", func(self *trCounter, amount int) int {
		self.value += int64(amount)
		return int(self.value)
	})
	cb.Method("get", func(self *trCounter) int {
		return int(self.value)
	})
	class := cb.Build()

	initMethod := class.Methods["__init__"].(*object.Builtin)
	instance := &object.Instance{Class: class, Fields: map[string]object.Object{}}

	result := initMethod.Fn(context.Background(), object.NewKwargs(nil), instance,
		object.NewInteger(10))
	if _, ok := result.(*object.Null); !ok {
		t.Fatalf("expected Null from __init__, got %T", result)
	}

	counter := instance.Fields["_receiver"].(*object.ClientWrapper).Client.(*trCounter)
	if counter.value != 10 {
		t.Errorf("expected initial value 10, got %d", counter.value)
	}

	incMethod := class.Methods["inc"].(*object.Builtin)
	result = incMethod.Fn(context.Background(), object.NewKwargs(nil), instance,
		object.NewInteger(5))
	if i, ok := result.(*object.Integer); !ok || i.IntValue() != 15 {
		t.Errorf("expected 15, got %v", result)
	}

	getMethod := class.Methods["get"].(*object.Builtin)
	result = getMethod.Fn(context.Background(), object.NewKwargs(nil), instance)
	if i, ok := result.(*object.Integer); !ok || i.IntValue() != 15 {
		t.Errorf("expected 15, got %v", result)
	}
}

func TestTypedReceiverDestructor(t *testing.T) {
	destroyed := false

	cb := object.NewClassBuilder("Resource")
	cb.Constructor(func(name string) *trConfig {
		return &trConfig{values: map[string]string{"name": name}}
	})
	cb.Method("get", func(self *trConfig, key string) string {
		return self.values[key]
	})
	cb.Method("__del__", func(self *trConfig) {
		destroyed = true
		self.values = nil
	})
	class := cb.Build()

	initMethod := class.Methods["__init__"].(*object.Builtin)
	instance := &object.Instance{Class: class, Fields: map[string]object.Object{}}
	initMethod.Fn(context.Background(), object.NewKwargs(nil), instance, object.NewString("db"))

	getMethod := class.Methods["get"].(*object.Builtin)
	result := getMethod.Fn(context.Background(), object.NewKwargs(nil), instance, object.NewString("name"))
	if str, ok := result.(*object.String); !ok || str.StringValue() != "db" {
		t.Errorf("expected 'db', got %v", result)
	}

	delMethod := class.Methods["__del__"].(*object.Builtin)
	delMethod.Fn(context.Background(), object.NewKwargs(nil), instance)
	if !destroyed {
		t.Error("expected __del__ to set destroyed flag")
	}
}

func TestTypedReceiverNoArgs(t *testing.T) {
	cb := object.NewClassBuilder("Simple")
	cb.Constructor(func() *trGreeter {
		return &trGreeter{greeting: "hello"}
	})
	cb.Method("greet", func(self *trGreeter) string {
		return self.greeting
	})
	class := cb.Build()

	initMethod := class.Methods["__init__"].(*object.Builtin)
	instance := &object.Instance{Class: class, Fields: map[string]object.Object{}}
	initMethod.Fn(context.Background(), object.NewKwargs(nil), instance)

	method := class.Methods["greet"].(*object.Builtin)
	result := method.Fn(context.Background(), object.NewKwargs(nil), instance)
	if str, ok := result.(*object.String); !ok || str.StringValue() != "hello" {
		t.Errorf("expected 'hello', got %v", result)
	}
}

func TestTypedReceiverReturnTypes(t *testing.T) {
	cb := object.NewClassBuilder("Types")
	cb.Constructor(func() *trConfig {
		return &trConfig{values: map[string]string{
			"count": "42",
		}}
	})
	cb.Method("get_string", func(self *trConfig) string {
		return "test"
	})
	cb.Method("get_int", func(self *trConfig) int {
		return 42
	})
	cb.Method("get_bool", func(self *trConfig) bool {
		return true
	})
	cb.Method("get_nil", func(self *trConfig) {
	})
	class := cb.Build()

	initMethod := class.Methods["__init__"].(*object.Builtin)
	instance := &object.Instance{Class: class, Fields: map[string]object.Object{}}
	initMethod.Fn(context.Background(), object.NewKwargs(nil), instance)

	strMethod := class.Methods["get_string"].(*object.Builtin)
	result := strMethod.Fn(context.Background(), object.NewKwargs(nil), instance)
	if s, ok := result.(*object.String); !ok || s.StringValue() != "test" {
		t.Errorf("expected 'test', got %v", result)
	}

	intMethod := class.Methods["get_int"].(*object.Builtin)
	result = intMethod.Fn(context.Background(), object.NewKwargs(nil), instance)
	if i, ok := result.(*object.Integer); !ok || i.IntValue() != 42 {
		t.Errorf("expected 42, got %v", result)
	}

	boolMethod := class.Methods["get_bool"].(*object.Builtin)
	result = boolMethod.Fn(context.Background(), object.NewKwargs(nil), instance)
	if b, ok := result.(*object.Boolean); !ok || !b.BoolValue() {
		t.Errorf("expected true, got %v", result)
	}

	nilMethod := class.Methods["get_nil"].(*object.Builtin)
	result = nilMethod.Fn(context.Background(), object.NewKwargs(nil), instance)
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("expected Null, got %T", result)
	}
}

func TestTypedReceiverContextKwargs(t *testing.T) {
	cb := object.NewClassBuilder("CtxKw")
	cb.Constructor(func() *trConfig {
		return &trConfig{values: make(map[string]string)}
	})
	cb.Method("set_kwargs", func(self *trConfig, ctx context.Context, kwargs object.Kwargs, key string, val string) {
		if ctx == nil {
			panic("context is nil")
		}
		self.values[key] = val
	})
	class := cb.Build()

	initMethod := class.Methods["__init__"].(*object.Builtin)
	instance := &object.Instance{Class: class, Fields: map[string]object.Object{}}
	initMethod.Fn(context.Background(), object.NewKwargs(nil), instance)

	method := class.Methods["set_kwargs"].(*object.Builtin)
	result := method.Fn(context.Background(), object.NewKwargs(nil), instance,
		object.NewString("k"), object.NewString("v"))
	if _, ok := result.(*object.Null); !ok {
		t.Errorf("expected Null, got %T", result)
	}

	cfg := instance.Fields["_receiver"].(*object.ClientWrapper).Client.(*trConfig)
	if cfg.values["k"] != "v" {
		t.Errorf("expected values['k'] = 'v', got %q", cfg.values["k"])
	}
}

func TestTypedReceiverErrorReturn(t *testing.T) {
	cb := object.NewClassBuilder("ErrTest")
	cb.Constructor(func() *trConfig {
		return &trConfig{values: make(map[string]string)}
	})
	cb.Method("maybe_fail", func(self *trConfig, fail bool) (string, error) {
		if fail {
			return "", fmt.Errorf("intentional error")
		}
		return "ok", nil
	})
	class := cb.Build()

	initMethod := class.Methods["__init__"].(*object.Builtin)
	instance := &object.Instance{Class: class, Fields: map[string]object.Object{}}
	initMethod.Fn(context.Background(), object.NewKwargs(nil), instance)

	method := class.Methods["maybe_fail"].(*object.Builtin)

	result := method.Fn(context.Background(), object.NewKwargs(nil), instance,
		object.NewBoolean(false))
	if s, ok := result.(*object.String); !ok || s.StringValue() != "ok" {
		t.Errorf("expected 'ok', got %v", result)
	}

	result = method.Fn(context.Background(), object.NewKwargs(nil), instance,
		object.NewBoolean(true))
	if errObj, ok := result.(*object.Error); !ok {
		t.Errorf("expected Error, got %T", result)
	} else if errObj.Message != "intentional error" {
		t.Errorf("expected 'intentional error', got %q", errObj.Message)
	}
}

func TestTypedReceiverStaticMethod(t *testing.T) {
	cb := object.NewClassBuilder("WithStatic")
	cb.Constructor(func() *trConfig {
		return &trConfig{values: make(map[string]string)}
	})
	cb.Method("get", func(self *trConfig) string {
		return "instance"
	})
	cb.StaticMethod("create", func(name string) string {
		return "static:" + name
	})
	class := cb.Build()

	sm := class.Methods["create"].(*object.StaticMethod)
	result := sm.Fn.(*object.Builtin).Fn(context.Background(), object.NewKwargs(nil), object.NewString("test"))
	if s, ok := result.(*object.String); !ok || s.StringValue() != "static:test" {
		t.Errorf("expected 'static:test', got %v", result)
	}
}

func TestTypedReceiverMultipleInstances(t *testing.T) {
	cb := object.NewClassBuilder("Multi")
	cb.Constructor(func(name string) *trConfig {
		return &trConfig{values: map[string]string{"name": name}}
	})
	cb.Method("get", func(self *trConfig, key string) string {
		return self.values[key]
	})
	class := cb.Build()

	initMethod := class.Methods["__init__"].(*object.Builtin)

	inst1 := &object.Instance{Class: class, Fields: map[string]object.Object{}}
	initMethod.Fn(context.Background(), object.NewKwargs(nil), inst1, object.NewString("first"))

	inst2 := &object.Instance{Class: class, Fields: map[string]object.Object{}}
	initMethod.Fn(context.Background(), object.NewKwargs(nil), inst2, object.NewString("second"))

	getMethod := class.Methods["get"].(*object.Builtin)

	r1 := getMethod.Fn(context.Background(), object.NewKwargs(nil), inst1, object.NewString("name"))
	if s, ok := r1.(*object.String); !ok || s.StringValue() != "first" {
		t.Errorf("expected 'first', got %v", r1)
	}

	r2 := getMethod.Fn(context.Background(), object.NewKwargs(nil), inst2, object.NewString("name"))
	if s, ok := r2.(*object.String); !ok || s.StringValue() != "second" {
		t.Errorf("expected 'second', got %v", r2)
	}
}

func TestTypedReceiverExplicitInitOverrides(t *testing.T) {
	called := false
	cb := object.NewClassBuilder("ExplicitInit")
	cb.Constructor(func() *trConfig {
		return &trConfig{values: map[string]string{}}
	})
	cb.Method("__init__", func(self *object.Instance) {
		called = true
		self.Fields["_receiver"] = &object.ClientWrapper{
			TypeName: "ExplicitInit",
			Client:   &trConfig{values: map[string]string{"explicit": "yes"}},
		}
	})
	cb.Method("get", func(self *trConfig, key string) string {
		return self.values[key]
	})
	class := cb.Build()

	initMethod := class.Methods["__init__"].(*object.Builtin)
	instance := &object.Instance{Class: class, Fields: map[string]object.Object{}}
	initMethod.Fn(context.Background(), object.NewKwargs(nil), instance)

	if !called {
		t.Error("expected explicit __init__ to be called")
	}

	getMethod := class.Methods["get"].(*object.Builtin)
	result := getMethod.Fn(context.Background(), object.NewKwargs(nil), instance, object.NewString("explicit"))
	if s, ok := result.(*object.String); !ok || s.StringValue() != "yes" {
		t.Errorf("expected 'yes', got %v", result)
	}
}
