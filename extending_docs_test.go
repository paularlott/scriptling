package scriptling

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// TestExtendingFunctionsDocs validates all examples from the Go Integration documentation
// See: https://scriptling.dev/docs/go-integration/native/functions/ and https://scriptling.dev/docs/go-integration/builder/functions/
func TestExtendingFunctionsDocs(t *testing.T) {
	t.Run("NativeAPI_SimpleFunction", func(t *testing.T) {
		p := New()
		p.RegisterFunc("double", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return &object.Error{Message: "double requires 1 argument"}
			}
			if intObj, ok := args[0].(*object.Integer); ok {
				return &object.Integer{Value: intObj.Value * 2}
			}
			return &object.Error{Message: "argument must be integer"}
		})

		_, err := p.Eval("result = double(21)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsInt("result")
		if objErr != nil || result != 42 {
			t.Errorf("expected 42, got %d", result)
		}
	})

	t.Run("NativeAPI_KeywordArgumentsOnly", func(t *testing.T) {
		p := New()
		p.RegisterFunc("make_duration", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) > 0 {
				return &object.Error{Message: "make_duration takes no positional arguments"}
			}

			hours, _ := kwargs.GetFloat("hours", 0.0)
			minutes, _ := kwargs.GetFloat("minutes", 0.0)
			seconds, _ := kwargs.GetFloat("seconds", 0.0)

			totalSeconds := hours*3600 + minutes*60 + seconds
			return &object.Float{Value: totalSeconds}
		})

		_, err := p.Eval("duration = make_duration(hours=2, minutes=30)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsFloat("duration")
		if objErr != nil || result != 9000.0 {
			t.Errorf("expected 9000.0, got %f", result)
		}
	})

	t.Run("NativeAPI_MixedPositionalAndKeyword", func(t *testing.T) {
		p := New()
		p.RegisterFunc("format_greeting", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return &object.Error{Message: "format_greeting requires name argument"}
			}

			name, err := args[0].AsString()
			if err != nil {
				return &object.Error{Message: "name must be string"}
			}

			prefix, _ := kwargs.GetString("prefix", "Hello")
			suffix, _ := kwargs.GetString("suffix", "!")

			return &object.String{Value: prefix + ", " + name + suffix}
		})

		tests := []struct {
			script   string
			expected string
		}{
			{`result = format_greeting("World")`, "Hello, World!"},
			{`result = format_greeting("World", prefix="Hi")`, "Hi, World!"},
			{`result = format_greeting("World", suffix="...")`, "Hello, World..."},
			{`result = format_greeting("World", prefix="Hey", suffix="?")`, "Hey, World?"},
		}

		for _, tt := range tests {
			_, err := p.Eval(tt.script)
			if err != nil {
				t.Fatalf("error: %v", err)
			}

			result, objErr := p.GetVarAsString("result")
			if objErr != nil || result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		}
	})

	t.Run("NativeAPI_VariadicArguments", func(t *testing.T) {
		p := New()
		p.RegisterFunc("sum_all", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			total := int64(0)
			for _, arg := range args {
				if intObj, ok := arg.(*object.Integer); ok {
					total += intObj.Value
				}
			}
			return &object.Integer{Value: total}
		})

		_, err := p.Eval("result = sum_all(1, 2, 3, 4, 5)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsInt("result")
		if objErr != nil || result != 15 {
			t.Errorf("expected 15, got %d", result)
		}
	})

	t.Run("NativeAPI_TypeSafeAccessor", func(t *testing.T) {
		p := New()
		p.RegisterFunc("add_tax", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 2 {
				return &object.Error{Message: "add_tax requires 2 arguments"}
			}

			price, errObj := args[0].AsFloat()
			if errObj != nil {
				return errObj
			}

			rate, errObj := args[1].AsFloat()
			if errObj != nil {
				return errObj
			}

			result := price * (1 + rate)
			return &object.Float{Value: result}
		})

		_, err := p.Eval("result = add_tax(100.0, 0.1)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsFloat("result")
		if objErr != nil || math.Abs(result-110.0) > 0.0001 {
			t.Errorf("expected ~110.0, got %f", result)
		}
	})

	t.Run("NativeAPI_WithHelpText", func(t *testing.T) {
		p := New()
		p.RegisterFunc("calculate", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Integer{Value: 42}
		}, `calculate(x, y) - Perform calculation

  Parameters:
    x - First number
    y - Second number

  Returns:
    The calculated result

  Examples:
    calculate(10, 5)  # Returns 15`)

		_, err := p.Eval(`help("calculate")`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("BuilderAPI_SimpleFunction", func(t *testing.T) {
		p := New()
		fb := object.NewFunctionBuilder()
		fb.Function(func(a, b int) int {
			return a + b
		})
		p.RegisterFunc("add", fb.Build())

		_, err := p.Eval("result = add(3, 4)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsInt("result")
		if objErr != nil || result != 7 {
			t.Errorf("expected 7, got %d", result)
		}
	})

	t.Run("BuilderAPI_WithContext", func(t *testing.T) {
		p := New()
		fb := object.NewFunctionBuilder()
		fb.Function(func(ctx context.Context, timeout int) string {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return "cancelled"
			default:
				return fmt.Sprintf("waited: %d", timeout)
			}
		})
		p.RegisterFunc("wait", fb.Build())

		_, err := p.Eval("result = wait(10)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
	})

	t.Run("BuilderAPI_WithKwargs", func(t *testing.T) {
		p := New()
		fb := object.NewFunctionBuilder()
		fb.Function(func(kwargs object.Kwargs) string {
			host := kwargs.MustGetString("host", "localhost")
			port := kwargs.MustGetInt("port", 8080)
			return fmt.Sprintf("%s:%d", host, port)
		})
		p.RegisterFunc("connect", fb.Build())

		_, err := p.Eval(`result = connect(host="example.com", port=443)`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsString("result")
		if objErr != nil || result != "example.com:443" {
			t.Errorf("expected 'example.com:443', got %s", result)
		}
	})

	t.Run("BuilderAPI_WithMixedArgs", func(t *testing.T) {
		p := New()
		fb := object.NewFunctionBuilder()
		fb.Function(func(kwargs object.Kwargs, name string, count int) string {
			prefix, _ := kwargs.GetString("prefix", ">")
			return fmt.Sprintf("%s %s: %d", prefix, name, count)
		})
		p.RegisterFunc("log", fb.Build())

		_, err := p.Eval(`result = log("task", 5, prefix=">>>")`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsString("result")
		if objErr != nil || result != ">>> task: 5" {
			t.Errorf("expected '>>> task: 5', got %s", result)
		}
	})

	t.Run("BuilderAPI_WithErrorReturn", func(t *testing.T) {
		p := New()
		fb := object.NewFunctionBuilder()
		fb.Function(func(a, b float64) (float64, error) {
			if b == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return a / b, nil
		})
		p.RegisterFunc("safe_divide", fb.Build())

		_, err := p.Eval("result = safe_divide(10.0, 2.0)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsFloat("result")
		if objErr != nil || result != 5.0 {
			t.Errorf("expected 5.0, got %f", result)
		}
	})

	t.Run("BuilderAPI_WithComplexTypes", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		fb := object.NewFunctionBuilder()
		fb.Function(func(items []any) float64 {
			sum := 0.0
			for _, item := range items {
				if v, ok := item.(float64); ok {
					sum += v
				}
			}
			return sum
		})
		p.RegisterFunc("sum_list", fb.Build())

		_, err := p.Eval("result = sum_list([1.0, 2.0, 3.0, 4.0])")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsFloat("result")
		if objErr != nil || result != 10.0 {
			t.Errorf("expected 10.0, got %f", result)
		}
	})

	t.Run("BuilderAPI_WithHelp", func(t *testing.T) {
		p := New()
		fb := object.NewFunctionBuilder()
		fb.FunctionWithHelp(func(x float64) float64 {
			return math.Sqrt(x)
		}, "sqrt(x) - Return the square root of x")
		p.RegisterFunc("sqrt", fb.Build())

		_, err := p.Eval("result = sqrt(16.0)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsFloat("result")
		if objErr != nil || result != 4.0 {
			t.Errorf("expected 4.0, got %f", result)
		}
	})
}

// TestExtendingLibrariesDocs validates all examples from the Go Integration documentation
// See: https://scriptling.dev/docs/go-integration/native/libraries/ and https://scriptling.dev/docs/go-integration/builder/libraries/
func TestExtendingLibrariesDocs(t *testing.T) {
	t.Run("NativeAPI_BasicLibrary", func(t *testing.T) {
		p := New()
		myLib := object.NewLibrary("mylib", map[string]*object.Builtin{
			"add": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					a, _ := args[0].(*object.Integer)
					b, _ := args[1].(*object.Integer)
					return &object.Integer{Value: a.Value + b.Value}
				},
				HelpText: "add(a, b) - Adds two numbers",
			},
			"multiply": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					a, _ := args[0].(*object.Integer)
					b, _ := args[1].(*object.Integer)
					return &object.Integer{Value: a.Value * b.Value}
				},
				HelpText: "multiply(a, b) - Multiplies two numbers",
			},
		}, nil, "My custom math library")
		p.RegisterLibrary( myLib)

		_, err := p.Eval(`
import mylib
result = mylib.add(5, 3)
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsInt("result")
		if objErr != nil || result != 8 {
			t.Errorf("expected 8, got %d", result)
		}
	})

	t.Run("NativeAPI_LibraryWithConstants", func(t *testing.T) {
		p := New()
		myLib := object.NewLibrary("mylib", 
			map[string]*object.Builtin{
				"add": {
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.Integer{Value: 0}
					},
					HelpText: "add(a, b) - Adds two numbers",
				},
			},
			map[string]object.Object{
				"VERSION":   &object.String{Value: "1.0.0"},
				"MAX_VALUE": &object.Integer{Value: 1000},
				"DEBUG":     &object.Boolean{Value: false},
			},
			"My custom math library",
		)
		p.RegisterLibrary( myLib)

		_, err := p.Eval(`
import mylib
version = mylib.VERSION
max = mylib.MAX_VALUE
debug = mylib.DEBUG
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		version, objErr := p.GetVarAsString("version")
		if objErr != nil || version != "1.0.0" {
			t.Errorf("expected 1.0.0, got %s", version)
		}

		max, objErr := p.GetVarAsInt("max")
		if objErr != nil || max != 1000 {
			t.Errorf("expected 1000, got %d", max)
		}
	})

	t.Run("NativeAPI_SubLibrary", func(t *testing.T) {
		p := New()

		// Create URL parsing sub-library
		parseLib := object.NewLibrary("url_parse", 
			map[string]*object.Builtin{
				"quote": {
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						s, _ := args[0].AsString()
						return &object.String{Value: "ENCODED+" + s}
					},
					HelpText: "quote(s) - URL encode a string",
				},
				"unquote": {
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						s, _ := args[0].AsString()
						return &object.String{Value: "DECODED-" + s}
					},
					HelpText: "unquote(s) - URL decode a string",
				},
			},
			nil,
			"URL parsing utilities",
		)

		// Create main URL library
		urlLib := object.NewLibrary("url", 
			map[string]*object.Builtin{
				"join": {
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						base, _ := args[0].AsString()
						path, _ := args[1].AsString()
						return &object.String{Value: strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/")}
					},
					HelpText: "join(base, path) - Join URL path segments",
				},
			},
			map[string]object.Object{
				"parse": parseLib,
			},
			"URL utilities",
		)
		p.RegisterLibrary( urlLib)
		p.RegisterLibrary( parseLib)

		_, err := p.Eval(`
import url
import url_parse
joined = url.join("https://example.com/", "/api/users")
quoted = url_parse.quote("hello")
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		joined, objErr := p.GetVarAsString("joined")
		if objErr != nil || joined != "https://example.com/api/users" {
			t.Errorf("expected 'https://example.com/api/users', got %s", joined)
		}

		quoted, objErr := p.GetVarAsString("quoted")
		if objErr != nil || quoted != "ENCODED+hello" {
			t.Errorf("expected 'ENCODED+hello', got %s", quoted)
		}
	})

	t.Run("NativeAPI_LibraryWithState", func(t *testing.T) {
		p := New()

		// Create a logger library that maintains state
		type Logger struct {
			level    string
			messages []string
		}

		_ = &Logger{
			level:    "INFO",
			messages: make([]string, 0),
		}

		loggerLib := object.NewLibrary("logger", 
			map[string]*object.Builtin{
				"set_level": {
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						// Note: This is a simplified version - real implementation would capture logger instance
						return &object.String{Value: "Level set"}
					},
					HelpText: "set_level(level) - Set the logging level",
				},
				"get_messages": {
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.List{Elements: []object.Object{
							&object.String{Value: "msg1"},
							&object.String{Value: "msg2"},
						}}
					},
					HelpText: "get_messages() - Get all logged messages",
				},
			},
			nil,
			"Logger library",
		)
		p.RegisterLibrary( loggerLib)

		_, err := p.Eval(`
import logger
msgs = logger.get_messages()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
	})

	t.Run("BuilderAPI_CreatingLibrary", func(t *testing.T) {
		p := New()

		builder := object.NewLibraryBuilder("mymath", "Mathematical operations library")

		// Register typed functions
		builder.Function("add", func(a, b int) int {
			return a + b
		})

		builder.Function("multiply", func(a, b float64) float64 {
			return a * b
		})

		// Register constants
		builder.Constant("PI", 3.14159)
		builder.Constant("MAX_VALUE", 1000)

		// Build the library
		myLib := builder.Build()
		p.RegisterLibrary( myLib)

		_, err := p.Eval(`
import mymath
sum_result = mymath.add(2, 3)
mult_result = mymath.multiply(4.0, 5.0)
pi = mymath.PI
max_val = mymath.MAX_VALUE
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		sumResult, objErr := p.GetVarAsInt("sum_result")
		if objErr != nil || sumResult != 5 {
			t.Errorf("expected 5, got %d", sumResult)
		}

		multResult, objErr := p.GetVarAsFloat("mult_result")
		if objErr != nil || multResult != 20.0 {
			t.Errorf("expected 20.0, got %f", multResult)
		}

		pi, objErr := p.GetVarAsFloat("pi")
		if objErr != nil || pi != 3.14159 {
			t.Errorf("expected 3.14159, got %f", pi)
		}
	})

	t.Run("BuilderAPI_WithKwargs", func(t *testing.T) {
		p := New()

		builder := object.NewLibraryBuilder("net", "Network utilities")

		builder.Function("connect", func(kwargs object.Kwargs) string {
			host := kwargs.MustGetString("host", "localhost")
			port := kwargs.MustGetInt("port", 8080)
			return fmt.Sprintf("%s:%d", host, port)
		})

		lib := builder.Build()
		p.RegisterLibrary( lib)

		_, err := p.Eval(`
import net
result = net.connect(host="example.com", port=443)
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsString("result")
		if objErr != nil || result != "example.com:443" {
			t.Errorf("expected 'example.com:443', got %s", result)
		}
	})

	t.Run("BuilderAPI_SubLibrary", func(t *testing.T) {
		p := New()

		// Create URL parsing sub-library
		parseBuilder := object.NewLibraryBuilder("url_parse", "URL parsing utilities")
		parseBuilder.Function("quote", func(s string) string {
			return "ENCODED:" + s
		})
		parseLib := parseBuilder.Build()

		// Create main URL library
		urlBuilder := object.NewLibraryBuilder("url", "URL utilities")
		urlBuilder.Function("join", func(base, path string) string {
			return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/")
		})
		urlBuilder.SubLibrary("parse", parseLib)
		urlLib := urlBuilder.Build()

		p.RegisterLibrary( urlLib)
		p.RegisterLibrary( parseLib)

		_, err := p.Eval(`
import url
import url_parse
joined = url.join("https://example.com/", "/api")
quoted = url_parse.quote("test")
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		joined, objErr := p.GetVarAsString("joined")
		if objErr != nil || joined != "https://example.com/api" {
			t.Errorf("expected 'https://example.com/api', got %s", joined)
		}

		quoted, objErr := p.GetVarAsString("quoted")
		if objErr != nil || quoted != "ENCODED:test" {
			t.Errorf("expected 'ENCODED:test', got %s", quoted)
		}
	})
}

// TestExtendingClassesDocs validates all examples from the Go Integration documentation
// See: https://scriptling.dev/docs/go-integration/native/classes/ and https://scriptling.dev/docs/go-integration/builder/classes/
func TestExtendingClassesDocs(t *testing.T) {
	t.Run("NativeAPI_BasicClass", func(t *testing.T) {
		p := New()

		// Create a simple Counter class
		counterClass := &object.Class{
			Name: "Counter",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						start := int64(0)
						if len(args) > 1 {
							if intObj, ok := args[1].(*object.Integer); ok {
								start = intObj.Value
							}
						}
						instance.Fields["count"] = &object.Integer{Value: start}
						return &object.Null{}
					},
					HelpText: "__init__(self, start=0) - Initialize counter",
				},
				"increment": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						count, _ := instance.Fields["count"].(*object.Integer)
						newCount := count.Value + 1
						instance.Fields["count"] = &object.Integer{Value: newCount}
						return &object.Integer{Value: newCount}
					},
					HelpText: "increment() - Increment and return new value",
				},
				"get": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						return instance.Fields["count"]
					},
					HelpText: "get() - Get current count",
				},
			},
		}

		// Register the class through a library to make it callable
		p.RegisterLibrary(object.NewLibrary("counters", nil, map[string]object.Object{
			"Counter": counterClass,
		}, "Counter library"))

		_, err := p.Eval(`
import counters
c = counters.Counter(10)
initial = c.get()
after = c.increment()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		initial, objErr := p.GetVarAsInt("initial")
		if objErr != nil || initial != 10 {
			t.Errorf("expected 10, got %d", initial)
		}

		after, objErr := p.GetVarAsInt("after")
		if objErr != nil || after != 11 {
			t.Errorf("expected 11, got %d", after)
		}
	})

	t.Run("NativeAPI_ClassWithLibrary", func(t *testing.T) {
		p := New()

		counterClass := &object.Class{
			Name: "Counter",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						instance.Fields["count"] = &object.Integer{Value: 0}
						return &object.Null{}
					},
					HelpText: "__init__(self) - Initialize counter",
				},
				"increment": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						count, _ := instance.Fields["count"].(*object.Integer)
						newVal := count.Value + 1
						instance.Fields["count"] = &object.Integer{Value: newVal}
						return &object.Integer{Value: newVal}
					},
					HelpText: "increment() - Increment counter",
				},
			},
		}

		counterLib := object.NewLibrary("counters", 
			map[string]*object.Builtin{
				"create_counter": {
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						// Create a new instance
						instance := &object.Instance{
							Class:  counterClass,
							Fields: map[string]object.Object{},
						}
						// Call __init__
						initMethod := counterClass.Methods["__init__"].(*object.Builtin)
						initMethod.Fn(ctx, object.NewKwargs(nil), instance)
						return instance
					},
					HelpText: "create_counter() - Create a new counter",
				},
			},
			map[string]object.Object{
				"Counter": counterClass,
			},
			"Counter library",
		)

		p.RegisterLibrary( counterLib)

		_, err := p.Eval(`
import counters
c = counters.Counter()
c.increment()
c.increment()
count = c.increment()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		count, objErr := p.GetVarAsInt("count")
		if objErr != nil || count != 3 {
			t.Errorf("expected 3, got %d", count)
		}
	})

	t.Run("NativeAPI_Inheritance", func(t *testing.T) {
		p := New()

		// Base class
		animalClass := &object.Class{
			Name: "Animal",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						name, _ := args[1].AsString()
						instance.Fields["name"] = &object.String{Value: name}
						return &object.Null{}
					},
					HelpText: "__init__(self, name) - Initialize animal",
				},
				"speak": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						name, _ := instance.Fields["name"].AsString()
						return &object.String{Value: name + " makes a sound"}
					},
					HelpText: "speak() - Make a sound",
				},
			},
		}

		// Derived class
		dogClass := &object.Class{
			Name:      "Dog",
			BaseClass: animalClass,
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						name, _ := args[1].AsString()

						// Call parent __init__
						parentInit := animalClass.Methods["__init__"].(*object.Builtin)
						parentInit.Fn(ctx, object.NewKwargs(nil), instance, &object.String{Value: name})

						instance.Fields["breed"] = &object.String{Value: "Unknown"}
						return &object.Null{}
					},
					HelpText: "__init__(self, name) - Initialize dog",
				},
				"bark": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						name, _ := instance.Fields["name"].AsString()
						return &object.String{Value: name + " says Woof!"}
					},
					HelpText: "bark() - Make the dog bark",
				},
			},
		}

		// Register classes through a library to make them callable
		p.RegisterLibrary(object.NewLibrary("animals", nil, map[string]object.Object{
			"Animal": animalClass,
			"Dog":    dogClass,
		}, "Animal library"))

		_, err := p.Eval(`
import animals
dog = animals.Dog("Buddy")
sound = dog.speak()
bark = dog.bark()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		sound, objErr := p.GetVarAsString("sound")
		if objErr != nil || sound != "Buddy makes a sound" {
			t.Errorf("expected 'Buddy makes a sound', got %s", sound)
		}

		bark, objErr := p.GetVarAsString("bark")
		if objErr != nil || bark != "Buddy says Woof!" {
			t.Errorf("expected 'Buddy says Woof!', got %s", bark)
		}
	})

	t.Run("BuilderAPI_BasicClass", func(t *testing.T) {
		p := New()

		cb := object.NewClassBuilder("Person")

		cb.Method("__init__", func(self *object.Instance, name string, age int) {
			self.Fields["name"] = &object.String{Value: name}
			self.Fields["age"] = &object.Integer{Value: int64(age)}
		})

		cb.Method("greet", func(self *object.Instance) string {
			name, _ := self.Fields["name"].AsString()
			return "Hello, I'm " + name
		})

		cb.Method("have_birthday", func(self *object.Instance) {
			age, _ := self.Fields["age"].AsInt()
			self.Fields["age"] = &object.Integer{Value: age + 1}
		})

		personClass := cb.Build()

		// Register the class through a library to make it callable
		p.RegisterLibrary(object.NewLibrary("people", nil, map[string]object.Object{
			"Person": personClass,
		}, "People library"))

		_, err := p.Eval(`
import people
p = people.Person("Alice", 30)
greeting = p.greet()
p.have_birthday()
new_age = p.age
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		greeting, objErr := p.GetVarAsString("greeting")
		if objErr != nil || greeting != "Hello, I'm Alice" {
			t.Errorf("expected 'Hello, I'm Alice', got %s", greeting)
		}

		newAge, objErr := p.GetVarAsInt("new_age")
		if objErr != nil || newAge != 31 {
			t.Errorf("expected 31, got %d", newAge)
		}
	})

	t.Run("CrossApproachInheritance_BuilderFromNative", func(t *testing.T) {
		p := New()

		// Native base class (Person)
		personClass := &object.Class{
			Name: "Person",
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						name, _ := args[1].AsString()
						age, _ := args[2].AsInt()
						instance.Fields["name"] = &object.String{Value: name}
						instance.Fields["age"] = &object.Integer{Value: age}
						return &object.Null{}
					},
					HelpText: "__init__(self, name, age) - Initialize Person",
				},
				"greet": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						name, _ := instance.Fields["name"].AsString()
						return &object.String{Value: "Hello, I'm " + name}
					},
					HelpText: "greet() - Return a greeting",
				},
			},
		}

		// Builder API derived class (Employee)
		cb := object.NewClassBuilder("Employee")
		cb.BaseClass(personClass)
		cb.Method("__init__", func(self *object.Instance, name string, age int, department string) {
			// Call parent __init__ using native API
			parentInit := personClass.Methods["__init__"].(*object.Builtin)
			parentInit.Fn(nil, object.NewKwargs(nil), self, &object.String{Value: name}, &object.Integer{Value: int64(age)})
			// Add employee-specific field
			self.Fields["department"] = &object.String{Value: department}
		})

		cb.Method("get_info", func(self *object.Instance) string {
			dept, _ := self.Fields["department"].AsString()
			age, _ := self.Fields["age"].AsInt()
			return fmt.Sprintf("Dept: %s, Age: %d", dept, age)
		})

		employeeClass := cb.Build()

		// Register both classes through a library to make them callable
		p.RegisterLibrary(object.NewLibrary("company", nil, map[string]object.Object{
			"Person":   personClass,
			"Employee": employeeClass,
		}, "Company library"))

		_, err := p.Eval(`
import company
emp = company.Employee("Bob", 35, "Engineering")
greeting = emp.greet()
info = emp.get_info()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		greeting, objErr := p.GetVarAsString("greeting")
		if objErr != nil || greeting != "Hello, I'm Bob" {
			t.Errorf("expected 'Hello, I'm Bob', got %s", greeting)
		}

		info, objErr := p.GetVarAsString("info")
		if objErr != nil || info != "Dept: Engineering, Age: 35" {
			t.Errorf("expected 'Dept: Engineering, Age: 35', got %s", info)
		}
	})

	t.Run("CrossApproachInheritance_NativeFromBuilder", func(t *testing.T) {
		p := New()

		// Builder base class (Animal)
		cb := object.NewClassBuilder("Animal")
		cb.Method("__init__", func(self *object.Instance, name string) {
			self.Fields["name"] = &object.String{Value: name}
		})
		cb.Method("speak", func(self *object.Instance) string {
			name, _ := self.Fields["name"].AsString()
			return name + " makes a sound"
		})
		animalClass := cb.Build()

		// Native derived class (Dog) inheriting from builder class
		dogClass := &object.Class{
			Name:      "Dog",
			BaseClass: animalClass,
			Methods: map[string]object.Object{
				"__init__": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						name, _ := args[1].AsString()

						// Call parent __init__ (from builder class)
						parentInit := animalClass.Methods["__init__"].(*object.Builtin)
						parentInit.Fn(ctx, object.NewKwargs(nil), instance, &object.String{Value: name})

						instance.Fields["breed"] = &object.String{Value: "Unknown"}
						return &object.Null{}
					},
					HelpText: "__init__(self, name) - Initialize Dog",
				},
				"bark": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						instance := args[0].(*object.Instance)
						name, _ := instance.Fields["name"].AsString()
						return &object.String{Value: name + " says Woof!"}
					},
					HelpText: "bark() - Make the dog bark",
				},
				// Inherit speak() from Animal (builder class)
				"speak": animalClass.Methods["speak"],
			},
		}

		// Register both classes through a library to make them callable
		p.RegisterLibrary(object.NewLibrary("pets", nil, map[string]object.Object{
			"Animal": animalClass,
			"Dog":    dogClass,
		}, "Pets library"))

		_, err := p.Eval(`
import pets
dog = pets.Dog("Rex")
sound = dog.speak()
bark = dog.bark()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		sound, objErr := p.GetVarAsString("sound")
		if objErr != nil || sound != "Rex makes a sound" {
			t.Errorf("expected 'Rex makes a sound', got %s", sound)
		}

		bark, objErr := p.GetVarAsString("bark")
		if objErr != nil || bark != "Rex says Woof!" {
			t.Errorf("expected 'Rex says Woof!', got %s", bark)
		}
	})

	t.Run("CrossApproachInheritance_BuilderFromBuilder", func(t *testing.T) {
		p := New()

		// Builder base class (Vehicle)
		vehicleBuilder := object.NewClassBuilder("Vehicle")
		vehicleBuilder.Method("__init__", func(self *object.Instance, make string, model string) {
			self.Fields["make"] = &object.String{Value: make}
			self.Fields["model"] = &object.String{Value: model}
		})
		vehicleBuilder.Method("get_info", func(self *object.Instance) string {
			make, _ := self.Fields["make"].AsString()
			model, _ := self.Fields["model"].AsString()
			return make + " " + model
		})
		vehicleClass := vehicleBuilder.Build()

		// Builder derived class (Car)
		carBuilder := object.NewClassBuilder("Car")
		carBuilder.BaseClass(vehicleClass)
		carBuilder.Method("__init__", func(self *object.Instance, make string, model string, doors int) {
			// Call parent __init__ using parent's built method
			parentInit := vehicleClass.Methods["__init__"].(*object.Builtin)
			parentInit.Fn(nil, object.NewKwargs(nil), self, &object.String{Value: make}, &object.String{Value: model})
			self.Fields["doors"] = &object.Integer{Value: int64(doors)}
		})

		carBuilder.Method("honk", func(self *object.Instance) string {
			make, _ := self.Fields["make"].AsString()
			return make + " goes beep beep!"
		})

		carClass := carBuilder.Build()

		// Register both classes through a library to make them callable
		p.RegisterLibrary(object.NewLibrary("vehicles", nil, map[string]object.Object{
			"Vehicle": vehicleClass,
			"Car":     carClass,
		}, "Vehicles library"))

		_, err := p.Eval(`
import vehicles
car = vehicles.Car("Toyota", "Camry", 4)
info = car.get_info()
honk = car.honk()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		info, objErr := p.GetVarAsString("info")
		if objErr != nil || info != "Toyota Camry" {
			t.Errorf("expected 'Toyota Camry', got %s", info)
		}

		honk, objErr := p.GetVarAsString("honk")
		if objErr != nil || honk != "Toyota goes beep beep!" {
			t.Errorf("expected 'Toyota goes beep beep!', got %s", honk)
		}
	})
}

// TestExtendingWithScriptsDocs validates all examples from the Go Integration documentation
// See: https://scriptling.dev/docs/go-integration/scripts/
func TestExtendingWithScriptsDocs(t *testing.T) {
	t.Run("RegisterScriptFunc_Simple", func(t *testing.T) {
		p := New()

		err := p.RegisterScriptFunc("calculate_area", `
def calculate_area(width, height):
    return width * height
calculate_area
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		_, err = p.Eval("result = calculate_area(10, 20)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsInt("result")
		if objErr != nil || result != 200 {
			t.Errorf("expected 200, got %d", result)
		}
	})

	t.Run("RegisterScriptFunc_DefaultParams", func(t *testing.T) {
		p := New()

		err := p.RegisterScriptFunc("format_name", `
def format_name(first, last, title="Mr."):
    return title + " " + first + " " + last
format_name
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		_, err = p.Eval(`
name1 = format_name("John", "Doe")
name2 = format_name("Jane", "Smith", "Dr.")
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		name1, objErr := p.GetVarAsString("name1")
		if objErr != nil || name1 != "Mr. John Doe" {
			t.Errorf("expected 'Mr. John Doe', got %s", name1)
		}

		name2, objErr := p.GetVarAsString("name2")
		if objErr != nil || name2 != "Dr. Jane Smith" {
			t.Errorf("expected 'Dr. Jane Smith', got %s", name2)
		}
	})

	t.Run("RegisterScriptFunc_Lambda", func(t *testing.T) {
		p := New()

		err := p.RegisterScriptFunc("double", `lambda x: x * 2`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		_, err = p.Eval("result = double(21)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsInt("result")
		if objErr != nil || result != 42 {
			t.Errorf("expected 42, got %d", result)
		}
	})

	t.Run("RegisterScriptFunc_Variadic", func(t *testing.T) {
		p := New()

		err := p.RegisterScriptFunc("sum_all", `
def sum_all(*args):
    total = 0
    for x in args:
        total = total + x
    return total
sum_all
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		_, err = p.Eval("result = sum_all(1, 2, 3, 4, 5)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		result, objErr := p.GetVarAsInt("result")
		if objErr != nil || result != 15 {
			t.Errorf("expected 15, got %d", result)
		}
	})

	t.Run("RegisterScriptLibrary_Basic", func(t *testing.T) {
		p := New()

		err := p.RegisterScriptLibrary("mathutils", `
def square(x):
    return x * x

def cube(x):
    return x * x * x

def sum_of_squares(a, b):
    return square(a) + square(b)

PI = 3.14159
E = 2.71828
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		_, err = p.Eval(`
import mathutils
sq = mathutils.square(5)
cb = mathutils.cube(3)
sum_sq = mathutils.sum_of_squares(3, 4)
pi = mathutils.PI
e = mathutils.E
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		sq, objErr := p.GetVarAsInt("sq")
		if objErr != nil || sq != 25 {
			t.Errorf("expected 25, got %d", sq)
		}

		cb, objErr := p.GetVarAsInt("cb")
		if objErr != nil || cb != 27 {
			t.Errorf("expected 27, got %d", cb)
		}

		sumSq, objErr := p.GetVarAsInt("sum_sq")
		if objErr != nil || sumSq != 25 {
			t.Errorf("expected 25, got %d", sumSq)
		}

		pi, objErr := p.GetVarAsFloat("pi")
		if objErr != nil || pi != 3.14159 {
			t.Errorf("expected 3.14159, got %f", pi)
		}
	})

	t.Run("RegisterScriptLibrary_WithClasses", func(t *testing.T) {
		p := New()

		err := p.RegisterScriptLibrary("shapes", `
class Rectangle:
    def __init__(self, width, height):
        self.width = width
        self.height = height

    def area(self):
        return self.width * self.height

    def perimeter(self):
        return 2 * (self.width + self.height)

class Circle:
    def __init__(self, radius):
        self.radius = radius

    def area(self):
        return 3.14159 * self.radius * self.radius
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		_, err = p.Eval(`
import shapes

rect = shapes.Rectangle(10, 20)
rect_area = rect.area()
rect_perim = rect.perimeter()

circ = shapes.Circle(5)
circ_area = circ.area()
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		rectArea, objErr := p.GetVarAsInt("rect_area")
		if objErr != nil || rectArea != 200 {
			t.Errorf("expected 200, got %d", rectArea)
		}

		rectPerim, objErr := p.GetVarAsInt("rect_perim")
		if objErr != nil || rectPerim != 60 {
			t.Errorf("expected 60, got %d", rectPerim)
		}

		circArea, objErr := p.GetVarAsFloat("circ_area")
		if objErr != nil || circArea != 78.53975 {
			t.Errorf("expected ~78.54, got %f", circArea)
		}
	})

	t.Run("RegisterScriptLibrary_NestedImports", func(t *testing.T) {
		p := New()

		// Register a base library
		err := p.RegisterScriptLibrary("geometry_base", `
def distance(x1, y1, x2, y2):
    dx = x2 - x1
    dy = y2 - y1
    return (dx * dx + dy * dy) ** 0.5
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		// Register a library that imports the base library
		err = p.RegisterScriptLibrary("geometry_advanced", `
import geometry_base

def circle_circumference(radius):
    return 2 * 3.14159 * radius

def distance_from_origin(x, y):
    return geometry_base.distance(0, 0, x, y)
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		_, err = p.Eval(`
import geometry_advanced
circ = geometry_advanced.circle_circumference(5)
dist = geometry_advanced.distance_from_origin(3, 4)
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		circ, objErr := p.GetVarAsFloat("circ")
		if objErr != nil || circ < 31.415 || circ > 31.416 {
			t.Errorf("expected ~31.4159, got %f", circ)
		}

		dist, objErr := p.GetVarAsFloat("dist")
		if objErr != nil || dist != 5.0 {
			t.Errorf("expected 5.0, got %f", dist)
		}
	})

	t.Run("RegisterScriptLibrary_WithGoLibrary", func(t *testing.T) {
		p := New()

		// Register a Go library
		p.RegisterLibrary(object.NewLibrary("gomath", map[string]*object.Builtin{
			"sqrt": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					if len(args) != 1 {
						return &object.Error{Message: "sqrt requires 1 argument"}
					}
					if num, ok := args[0].(*object.Float); ok {
						return &object.Float{Value: math.Sqrt(num.Value)}
					}
					return &object.Error{Message: "argument must be float"}
				},
			},
		}, nil, "Custom mathematical functions library"))

		// Register a Scriptling library that uses the Go library
		err := p.RegisterScriptLibrary("advanced_math", `
import gomath

def pythagorean(a, b):
    c_squared = a * a + b * b
    return gomath.sqrt(c_squared)

def distance_3d(x1, y1, z1, x2, y2, z2):
    dx = x2 - x1
    dy = y2 - y1
    dz = z2 - z1
    return gomath.sqrt(dx*dx + dy*dy + dz*dz)
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		_, err = p.Eval(`
import advanced_math
hyp = advanced_math.pythagorean(3.0, 4.0)
dist = advanced_math.distance_3d(0.0, 0.0, 0.0, 1.0, 2.0, 2.0)
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		hyp, objErr := p.GetVarAsFloat("hyp")
		if objErr != nil || hyp != 5.0 {
			t.Errorf("expected 5.0, got %f", hyp)
		}

		dist, objErr := p.GetVarAsFloat("dist")
		if objErr != nil || dist != 3.0 {
			t.Errorf("expected 3.0, got %f", dist)
		}
	})

	t.Run("RegisterScriptLibrary_WithStandardLibrary", func(t *testing.T) {
		p := New()
		p.RegisterLibrary( stdlib.JSONLibrary)

		// Register a library that uses the json standard library
		err := p.RegisterScriptLibrary("data_processor", `
import json

def parse_user(json_str):
    user = json.loads(json_str)
    return user["name"] + " (" + str(user["age"]) + ")"

def create_user_json(name, age):
    data = {"name": name, "age": age}
    return json.dumps(data)
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		_, err = p.Eval(`
import data_processor
user_json = data_processor.create_user_json("Alice", 30)
parsed = data_processor.parse_user(user_json)
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		parsed, objErr := p.GetVarAsString("parsed")
		if objErr != nil || parsed != "Alice (30)" {
			t.Errorf("expected 'Alice (30)', got %s", parsed)
		}
	})
}

// TestKwargsHelpers validates all Kwargs helper methods from documentation
func TestKwargsHelpers(t *testing.T) {
	t.Run("GetStringWithDefault", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			val, errObj := kwargs.GetString("name", "default")
			if errObj != nil {
				return errObj
			}
			return &object.String{Value: val}
		})

		// Test with default
		_, err := p.Eval("result = test()")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsString("result")
		if result != "default" {
			t.Errorf("expected 'default', got %s", result)
		}

		// Test with value
		_, err = p.Eval(`result = test(name="custom")`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ = p.GetVarAsString("result")
		if result != "custom" {
			t.Errorf("expected 'custom', got %s", result)
		}
	})

	t.Run("GetIntWithDefault", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			val, errObj := kwargs.GetInt("count", 10)
			if errObj != nil {
				return errObj
			}
			return &object.Integer{Value: val}
		})

		_, err := p.Eval("result = test()")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsInt("result")
		if result != 10 {
			t.Errorf("expected 10, got %d", result)
		}

		_, err = p.Eval("result = test(count=42)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ = p.GetVarAsInt("result")
		if result != 42 {
			t.Errorf("expected 42, got %d", result)
		}
	})

	t.Run("GetFloatWithDefault", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			val, errObj := kwargs.GetFloat("rate", 1.0)
			if errObj != nil {
				return errObj
			}
			return &object.Float{Value: val}
		})

		_, err := p.Eval("result = test()")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsFloat("result")
		if result != 1.0 {
			t.Errorf("expected 1.0, got %f", result)
		}

		_, err = p.Eval("result = test(rate=2.5)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ = p.GetVarAsFloat("result")
		if result != 2.5 {
			t.Errorf("expected 2.5, got %f", result)
		}
	})

	t.Run("GetBoolWithDefault", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			val, errObj := kwargs.GetBool("enabled", false)
			if errObj != nil {
				return errObj
			}
			return &object.Boolean{Value: val}
		})

		_, err := p.Eval("result = test()")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsBool("result")
		if result != false {
			t.Errorf("expected false, got %t", result)
		}

		_, err = p.Eval("result = test(enabled=True)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ = p.GetVarAsBool("result")
		if result != true {
			t.Errorf("expected true, got %t", result)
		}
	})

	t.Run("HasHelper", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			hasA := kwargs.Has("a")
			hasB := kwargs.Has("b")
			return &object.Boolean{Value: hasA && !hasB}
		})

		_, err := p.Eval(`result = test(a="value")`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsBool("result")
		if !result {
			t.Errorf("expected true (has a, not b)")
		}
	})

	t.Run("KeysHelper", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			keys := kwargs.Keys()
			elements := make([]object.Object, len(keys))
			for i, key := range keys {
				elements[i] = &object.String{Value: key}
			}
			return &object.List{Elements: elements}
		})

		_, err := p.Eval(`result = test(a="1", b="2", c="3")`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsList("result")
		// Keys are not ordered, so just check we got 3 keys
		if len(result) != 3 {
			t.Errorf("expected 3 keys, got %d", len(result))
		}
	})

	t.Run("LenHelper", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Integer{Value: int64(kwargs.Len())}
		})

		_, err := p.Eval(`result = test(a="1", b="2", c="3")`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsInt("result")
		if result != 3 {
			t.Errorf("expected 3, got %d", result)
		}
	})

	t.Run("MustHelpers", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			s := kwargs.MustGetString("str", "default")
			i := kwargs.MustGetInt("int", 42)
			return &object.String{Value: fmt.Sprintf("%s:%d", s, i)}
		})

		_, err := p.Eval(`result = test(str="hello", int=100)`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsString("result")
		if result != "hello:100" {
			t.Errorf("expected 'hello:100', got %s", result)
		}

		// Test defaults
		_, err = p.Eval("result = test()")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ = p.GetVarAsString("result")
		if result != "default:42" {
			t.Errorf("expected 'default:42', got %s", result)
		}
	})
}

// TestTypeSafeAccessors validates all As* accessor methods
func TestTypeSafeAccessors(t *testing.T) {
	t.Run("AsString", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if len(args) != 1 {
				return &object.Error{Message: "need 1 arg"}
			}
			val, errObj := args[0].AsString()
			if errObj != nil {
				return errObj
			}
			return &object.String{Value: "got: " + val}
		})

		_, err := p.Eval(`result = test("hello")`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsString("result")
		if result != "got: hello" {
			t.Errorf("expected 'got: hello', got %s", result)
		}

		// Test with integer (should fail - AsString is type-safe)
		_, err = p.Eval("result = test(123)")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "must be a string") {
			t.Errorf("expected 'must be a string' error, got %v", err)
		}
	})

	t.Run("AsInt", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			val, errObj := args[0].AsInt()
			if errObj != nil {
				return errObj
			}
			return &object.Integer{Value: val + 1}
		})

		// Integer should work
		_, err := p.Eval("result = test(10)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsInt("result")
		if result != 11 {
			t.Errorf("expected 11, got %d", result)
		}

		// Float should truncate
		_, err = p.Eval("result = test(10.8)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ = p.GetVarAsInt("result")
		if result != 11 {
			t.Errorf("expected 11, got %d", result)
		}
	})

	t.Run("AsFloat", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			val, errObj := args[0].AsFloat()
			if errObj != nil {
				return errObj
			}
			return &object.Float{Value: val * 2}
		})

		// Float should work
		_, err := p.Eval("result = test(2.5)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsFloat("result")
		if result != 5.0 {
			t.Errorf("expected 5.0, got %f", result)
		}

		// Integer should convert
		_, err = p.Eval("result = test(3)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ = p.GetVarAsFloat("result")
		if result != 6.0 {
			t.Errorf("expected 6.0, got %f", result)
		}
	})

	t.Run("AsBool", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			val, errObj := args[0].AsBool()
			if errObj != nil {
				return errObj
			}
			return &object.Boolean{Value: !val}
		})

		_, err := p.Eval("result = test(True)")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsBool("result")
		if result != false {
			t.Errorf("expected false, got %t", result)
		}

		// String should work (empty string is false)
		_, err = p.Eval(`result = test("")`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ = p.GetVarAsBool("result")
		if result != true {
			t.Errorf("expected true, got %t", result)
		}
	})

	t.Run("AsList", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			val, errObj := args[0].AsList()
			if errObj != nil {
				return errObj
			}
			return &object.Integer{Value: int64(len(val))}
		})

		_, err := p.Eval("result = test([1, 2, 3])")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsInt("result")
		if result != 3 {
			t.Errorf("expected 3, got %d", result)
		}
	})

	t.Run("AsDict", func(t *testing.T) {
		p := New()
		p.RegisterFunc("test", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			val, errObj := args[0].AsDict()
			if errObj != nil {
				return errObj
			}
			return &object.Integer{Value: int64(len(val))}
		})

		_, err := p.Eval(`result = test({"a": 1, "b": 2})`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		result, _ := p.GetVarAsInt("result")
		if result != 2 {
			t.Errorf("expected 2, got %d", result)
		}
	})
}

// TestHelpSystemFromDocs validates help system examples from documentation
func TestHelpSystemFromDocs(t *testing.T) {
	t.Run("HelpForNativeFunction", func(t *testing.T) {
		p := New()
		p.RegisterFunc("my_func", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return &object.Null{}
		}, `my_func() - Description of the function

Detailed documentation here.`)

		_, err := p.Eval(`help("my_func")`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("HelpForLibraryFunction", func(t *testing.T) {
		p := New()
		myLib := object.NewLibrary("mylib", map[string]*object.Builtin{
			"validate": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return &object.Boolean{Value: true}
				},
				HelpText: `validate(email) - Validate email address

  Parameters:
    email - Email address to validate

  Returns:
    True if valid, False otherwise

  Examples:
    mylib.validate("test@example.com")`,
			},
		}, nil, "My custom data processing library")
		p.RegisterLibrary( myLib)

		_, err := p.Eval(`
import mylib
help("mylib.validate")
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("HelpForScriptLibraryWithDocstring", func(t *testing.T) {
		p := New()
		err := p.RegisterScriptLibrary("mylib", `
"""My Library - Custom data processing utilities

This library provides functions for data processing and formatting.
"""

def process(data):
    """Process input data.

    Args:
        data: Input string or list

    Returns:
        Processed data
    """
    if isinstance(data, str):
        return data.upper()
    return data

def format(value, fmt_type="default"):
    """Format a value for display.

    Args:
        value: Value to format
        fmt_type: Format type (default: "default")

    Returns:
        Formatted string
    """
    return str(value)
`)
		if err != nil {
			t.Fatalf("failed to register script library: %v", err)
		}

		_, err = p.Eval(`
import mylib
help("mylib")
`)
		if err != nil {
			t.Errorf("help for library failed: %v", err)
		}
	})
}

// TestCompleteExamplesFromDocs validates complete working examples
func TestCompleteExamplesFromDocs(t *testing.T) {
	t.Run("CompleteMathLibraryExample", func(t *testing.T) {
		p := New()

		builder := object.NewLibraryBuilder("mymath", "Advanced math operations")

		// Basic operations
		builder.Function("add", func(a, b int) int {
			return a + b
		})

		builder.FunctionWithHelp("multiply", func(a, b float64) float64 {
			return a * b
		}, "multiply(a, b) - Multiply two numbers")

		// Advanced operations with error handling
		builder.FunctionWithHelp("divide", func(a, b float64) (float64, error) {
			if b == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			return a / b, nil
		}, "divide(a, b) - Divide two numbers (returns error if b is zero)")

		builder.Function("sqrt", func(x float64) float64 {
			return math.Sqrt(x)
		})

		builder.Function("power", func(base, exp float64) float64 {
			return math.Pow(base, exp)
		})

		// Constants
		builder.Constant("PI", math.Pi)
		builder.Constant("E", math.E)
		builder.Constant("GoldenRatio", 1.618)

		// Build and register the library
		myMath := builder.Build()
		p.RegisterLibrary( myMath)

		// Use the library
		_, err := p.Eval(`
import mymath

# Basic operations
sum_result = mymath.add(2, 3)
mult_result = mymath.multiply(4.0, 5.0)
div_result = mymath.divide(10.0, 2.0)
sqrt_result = mymath.sqrt(16.0)
power_result = mymath.power(2.0, 8.0)

# Constants
pi_val = mymath.PI
e_val = mymath.E
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		sumResult, _ := p.GetVarAsInt("sum_result")
		if sumResult != 5 {
			t.Errorf("expected 5, got %d", sumResult)
		}

		multResult, _ := p.GetVarAsFloat("mult_result")
		if multResult != 20.0 {
			t.Errorf("expected 20.0, got %f", multResult)
		}

		divResult, _ := p.GetVarAsFloat("div_result")
		if divResult != 5.0 {
			t.Errorf("expected 5.0, got %f", divResult)
		}

		sqrtResult, _ := p.GetVarAsFloat("sqrt_result")
		if sqrtResult != 4.0 {
			t.Errorf("expected 4.0, got %f", sqrtResult)
		}

		powerResult, _ := p.GetVarAsFloat("power_result")
		if powerResult != 256.0 {
			t.Errorf("expected 256.0, got %f", powerResult)
		}

		piVal, _ := p.GetVarAsFloat("pi_val")
		if piVal != math.Pi {
			t.Errorf("expected PI, got %f", piVal)
		}
	})

	t.Run("CompletePersonClassExample", func(t *testing.T) {
		p := New()

		cb := object.NewClassBuilder("Person")

		cb.Method("__init__", func(self *object.Instance, name string, age int) {
			self.Fields["name"] = &object.String{Value: name}
			self.Fields["age"] = &object.Integer{Value: int64(age)}
		})

		cb.Method("greet", func(self *object.Instance) string {
			name, _ := self.Fields["name"].AsString()
			return "Hello, I'm " + name
		})

		cb.Method("have_birthday", func(self *object.Instance) {
			age, _ := self.Fields["age"].AsInt()
			self.Fields["age"] = &object.Integer{Value: age + 1}
		})

		personClass := cb.Build()

		// Register the class through a library to make it callable
		p.RegisterLibrary(object.NewLibrary("people", nil, map[string]object.Object{
			"Person": personClass,
		}, "People library"))

		_, err := p.Eval(`
import people
p = people.Person("Charlie", 25)
greeting = p.greet()
original_age = p.age
p.have_birthday()
new_age = p.age
`)
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		greeting, _ := p.GetVarAsString("greeting")
		if greeting != "Hello, I'm Charlie" {
			t.Errorf("expected 'Hello, I'm Charlie', got %s", greeting)
		}

		originalAge, _ := p.GetVarAsInt("original_age")
		if originalAge != 25 {
			t.Errorf("expected 25, got %d", originalAge)
		}

		newAge, _ := p.GetVarAsInt("new_age")
		if newAge != 26 {
			t.Errorf("expected 26, got %d", newAge)
		}
	})
}

// TestHelpSystemComprehensive validates all help system examples from the Go Integration documentation
// See: https://scriptling.dev/docs/go-integration/documentation/
func TestHelpSystemComprehensive(t *testing.T) {
	t.Run("UserFunctionDocstrings", func(t *testing.T) {
		p := New()

		// Test docstring extraction from user-defined functions
		_, err := p.Eval(`
def calculate_area(radius):
    """Calculate the area of a circle.

    Parameters:
      radius - The radius of the circle

    Returns:
      The area as a float
    """
    return 3.14159 * radius * radius

# Test help on the function
help(calculate_area)
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("FunctionBuilderWithHelp", func(t *testing.T) {
		p := New()

		// Use the builder pattern to create a function with help
		fb := object.NewFunctionBuilder()
		fn := fb.FunctionWithHelp(func(a, b int) int {
			return a + b
		}, `my_func(arg1, arg2) - Brief description

  Detailed description of what the function does.

  Parameters:
    arg1 - Description of first parameter
    arg2 - Description of second parameter

  Returns:
    Description of return value`).
			Build()

		p.RegisterFunc("my_func", fn)

		_, err := p.Eval(`help("my_func")`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("LibraryBuilderWithDescription", func(t *testing.T) {
		p := New()

		// Build a library with description and functions
		library := object.NewLibraryBuilder("mylib", "My custom data processing library").
			FunctionWithHelp("process", func(data string) string {
				return "processed: " + data
			}, `process(data) - Process the input data

  Takes input data and processes it.

  Parameters:
    data - The data to process

  Returns:
    Processed data as string`).
			Build()

		p.RegisterLibrary( library)

		_, err := p.Eval(`
import mylib
help("mylib")
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("ClassBuilderWithHelp", func(t *testing.T) {
		p := New()

		// Build a class with methods
		classBuilder := object.NewClassBuilder("MyClass")
		classBuilder.Method("__init__", func(self *object.Instance, name string) {
			self.Fields["name"] = &object.String{Value: name}
		})
		classBuilder.Method("get_data", func(self *object.Instance) string {
			name, _ := self.Fields["name"].AsString()
			return "data for " + name
		})
		myClass := classBuilder.Build()

		// Register through a library
		p.RegisterLibrary(object.NewLibrary("mylib", nil, map[string]object.Object{
			"MyClass": myClass,
		}, "My library"))

		_, err := p.Eval(`
import mylib
help(mylib.MyClass)
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("RegisterScriptFuncWithDocstring", func(t *testing.T) {
		p := New()

		// Register a function from script with docstring
		script := `
def process_data(data):
    """Process input data and return result.

    Args:
        data: The data to process

    Returns:
        Processed data
    """
    return data.upper()
`
		err := p.RegisterScriptFunc("process_data", script)
		if err != nil {
			t.Fatalf("failed to register script func: %v", err)
		}

		_, err = p.Eval(`help("process_data")`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("LambdaFunctionHelp", func(t *testing.T) {
		p := New()

		// Test help on lambda function
		_, err := p.Eval(`
my_lambda = lambda x: x * 2
help(my_lambda)
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("InstanceHelp", func(t *testing.T) {
		p := New()

		// Test help on class instance
		_, err := p.Eval(`
class Dog:
    def __init__(self, name):
        self.name = name

    def bark(self):
        return self.name + " barks!"

d = Dog("Buddy")
help(d)
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("BuiltinObjectHelp", func(t *testing.T) {
		p := New()

		// Test help on builtin object (not string name)
		_, err := p.Eval(`
help(print)
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("HelpOnScriptFunctionByName", func(t *testing.T) {
		p := New()

		// Test help by function name (string) vs object
		_, err := p.Eval(`
def my_function(a, b=10):
    """Add two numbers together."""
    return a + b

help("my_function")
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("LibraryModuleDocstring", func(t *testing.T) {
		p := New()

		// Register a script library with module docstring
		err := p.RegisterScriptLibrary("testlib", `
"""My Library - Custom utilities.

This library provides helper functions.
"""

def helper(x):
    """Helper function."""
    return x * 2
`)
		if err != nil {
			t.Fatalf("failed to register: %v", err)
		}

		_, err = p.Eval(`
import testlib
help("testlib")
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("HelpOperatorsAndBuiltins", func(t *testing.T) {
		p := New()

		// Test help for special topics
		_, err := p.Eval(`
help("operators")
help("builtins")
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("HelpAfterImport", func(t *testing.T) {
		p := New()

		// Register a library
		lib := object.NewLibrary("custom", nil, map[string]object.Object{
			"custom_func": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return &object.String{Value: "result"}
				},
				HelpText: `custom_func(arg) - Custom function description`,
			},
		}, "My custom library")

		p.RegisterLibrary( lib)

		// Test help after import
		_, err := p.Eval(`
import custom
help("custom.custom_func")
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("FunctionWithDefaultParametersHelp", func(t *testing.T) {
		p := New()

		// Test help shows default parameter values
		_, err := p.Eval(`
def greet(name, greeting="Hello", times=1):
    """Greet someone multiple times."""
    result = ""
    for i in range(times):
        result += greeting + " " + name + "! "
    return result

help(greet)
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})

	t.Run("VariadicFunctionHelp", func(t *testing.T) {
		p := New()

		// Test help for variadic functions
		_, err := p.Eval(`
def sum_all(*numbers):
    """Sum all numbers."""
    total = 0
    for n in numbers:
        total += n
    return total

help(sum_all)
`)
		if err != nil {
			t.Errorf("help failed: %v", err)
		}
	})
}
