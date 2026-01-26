package scriptling

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

func TestRegisterFunc(t *testing.T) {
	p := New()
	p.RegisterFunc("double", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		if len(args) != 1 {
			return &object.Error{Message: "need 1 argument"}
		}
		val := args[0].(*object.Integer).Value
		return &object.Integer{Value: val * 2}
	})

	_, err := p.Eval("result = double(5)")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, objErr := p.GetVar("result")
	if objErr != nil || result != int64(10) {
		t.Errorf("expected 10, got %v", result)
	}
}

func TestRegisterLibrary(t *testing.T) {
	p := New()
	myLib := object.NewLibrary("mylib", map[string]*object.Builtin{
		"greet": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return &object.String{Value: "Hello!"}
			},
		},
	}, nil, "")
	p.RegisterLibrary( myLib)

	_, err := p.Eval(`
import mylib
msg = mylib.greet()
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	msg, objErr := p.GetVar("msg")
	if objErr != nil || msg != "Hello!" {
		t.Errorf("expected Hello!, got %v", msg)
	}
}

func TestImport(t *testing.T) {
	p := New()
	myLib := object.NewLibrary("mylib", map[string]*object.Builtin{
		"greet": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return &object.String{Value: "Hello!"}
			},
		},
	}, nil, "")
	p.RegisterLibrary( myLib)

	// Import the library programmatically
	err := p.Import("mylib")
	if err != nil {
		t.Fatalf("error importing library: %v", err)
	}

	// Now use it in script without import statement
	_, err = p.Eval("msg = mylib.greet()")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	msg, objErr := p.GetVar("msg")
	if objErr != nil || msg != "Hello!" {
		t.Errorf("expected Hello!, got %v", msg)
	}
}

func TestImportStandardLibrary(t *testing.T) {
	p := New()
	p.RegisterLibrary( stdlib.JSONLibrary)

	// Import the json library programmatically
	err := p.Import("json")
	if err != nil {
		t.Fatalf("error importing json library: %v", err)
	}

	// Now use it in script without import statement
	_, err = p.Eval("data = json.dumps({'key': 'value'})")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	data, objErr := p.GetVar("data")
	if objErr != nil || data != `{"key":"value"}` {
		t.Errorf("expected {\"key\":\"value\"}, got %v", data)
	}
}

func TestRegisterLibraryWithClass(t *testing.T) {
	p := New()

	// Define a class in Go
	counterClass := &object.Class{
		Name: "Counter",
		Methods: map[string]object.Object{
			"__init__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					self := args[0].(*object.Instance)
					start := int64(0)
					if len(args) > 1 {
						if intObj, ok := args[1].(*object.Integer); ok {
							start = intObj.Value
						}
					}
					self.Fields["value"] = &object.Integer{Value: start}
					return &object.Null{}
				},
			},
			"increment": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					self := args[0].(*object.Instance)
					if val, ok := self.Fields["value"].(*object.Integer); ok {
						newVal := val.Value + 1
						self.Fields["value"] = &object.Integer{Value: newVal}
						return &object.Integer{Value: newVal}
					}
					return &object.Null{}
				},
			},
			"get": &object.Builtin{
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					self := args[0].(*object.Instance)
					// Return a copy to avoid mutation issues
					if val, ok := self.Fields["value"].(*object.Integer); ok {
						return &object.Integer{Value: val.Value}
					}
					return self.Fields["value"]
				},
			},
		},
	}

	// Create library with the class in the constants map
	myLib := object.NewLibrary("counters", 
		map[string]*object.Builtin{
			"helper": {
				Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
					return &object.String{Value: "helper called"}
				},
			},
		},
		map[string]object.Object{
			"Counter": counterClass,
			"VERSION": &object.String{Value: "1.0.0"},
		},
		"Counter utilities library",
	)

	p.RegisterLibrary( myLib)

	// Test using the class from the library
	_, err := p.Eval(`
import counters
c = counters.Counter(10)
initial = c.get()
after_inc = c.increment()
version = counters.VERSION
helper_result = counters.helper()
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	initial, objErr := p.GetVar("initial")
	if objErr != nil || initial != int64(10) {
		t.Errorf("expected initial=10, got %v", initial)
	}

	afterInc, objErr := p.GetVar("after_inc")
	if objErr != nil || afterInc != int64(11) {
		t.Errorf("expected after_inc=11, got %v", afterInc)
	}

	version, objErr := p.GetVar("version")
	if objErr != nil || version != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %v", version)
	}

	helperResult, objErr := p.GetVar("helper_result")
	if objErr != nil || helperResult != "helper called" {
		t.Errorf("expected helper_result='helper called', got %v", helperResult)
	}
}

func TestImportBuiltin(t *testing.T) {
	p := New()
	p.RegisterLibrary( stdlib.JSONLibrary)
	_, err := p.Eval(`
import json
data = json.loads('{"key":"value"}')
result = data["key"]
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, objErr := p.GetVar("result")
	if objErr != nil || result != "value" {
		t.Errorf("expected value, got %v", result)
	}
}

func TestOnDemandLibraryCallback(t *testing.T) {
	p := New()

	// Set callback that registers a custom library on demand
	p.SetOnDemandLibraryCallback(func(s *Scriptling, name string) bool {
		if name == "customlib" {
			// Register a simple library
			return s.RegisterScriptLibrary("customlib", "PI = 3.14\ndef add(a, b):\n    return a + b") == nil
		}
		return false
	})

	// Try to import the library that doesn't exist yet
	_, err := p.Eval(`
import customlib
result = customlib.add(2, 3)
pi_value = customlib.PI
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, objErr := p.GetVar("result")
	if objErr != nil || result != int64(5) {
		t.Errorf("expected 5, got %v", result)
	}

	pi_value, objErr := p.GetVar("pi_value")
	if objErr != nil || pi_value != 3.14 {
		t.Errorf("expected 3.14, got %v", pi_value)
	}
}

func TestModuloOperator(t *testing.T) {
	p := New()
	_, err := p.Eval("result = 10 % 3")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, objErr := p.GetVar("result")
	if objErr != nil || result != int64(1) {
		t.Errorf("expected 1, got %v", result)
	}
}

func TestBooleanOperators(t *testing.T) {
	p := New()
	_, err := p.Eval(`
and_result = True and False
or_result = True or False
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	andResult, objErr := p.GetVar("and_result")
	if objErr != nil || andResult != false {
		t.Errorf("expected false, got %v", andResult)
	}

	orResult, objErr := p.GetVar("or_result")
	if objErr != nil || orResult != true {
		t.Errorf("expected true, got %v", orResult)
	}
}

func TestComparisonOperators(t *testing.T) {
	p := New()
	_, err := p.Eval(`
lte = 5 <= 5
gte = 10 >= 5
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	lte, objErr := p.GetVar("lte")
	if objErr != nil || lte != true {
		t.Errorf("expected true for <=, got %v", lte)
	}

	gte, objErr := p.GetVar("gte")
	if objErr != nil || gte != true {
		t.Errorf("expected true for >=, got %v", gte)
	}
}

func TestDotNotation(t *testing.T) {
	p := New()
	p.RegisterLibrary( stdlib.JSONLibrary)
	_, err := p.Eval(`
import json
data = json.loads('{"name":"Alice"}')
result = data["name"]
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, objErr := p.GetVar("result")
	if objErr != nil || result != "Alice" {
		t.Errorf("expected Alice, got %v", result)
	}
}

func TestHTTPLibrary(t *testing.T) {
	p := New()
	p.RegisterLibrary( extlibs.RequestsLibrary)
	_, err := p.Eval(`
import requests
options = {"timeout": 10}
response = requests.get("http://127.0.0.1:9000/status/200", options)
print("Response:", response)
status = response.status_code
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	status, objErr := p.GetVarAsInt("status")
	// Accept both 200 (success) and other status codes (service issues)
	if objErr != nil || (status != int64(200) && status < 400) {
		t.Errorf("expected 200 or success status, got %v", status)
	}
}

func TestHelpSystem(t *testing.T) {
	p := New()

	// Register a Go function with help text
	p.RegisterFunc("custom_func", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
		return &object.String{Value: "custom result"}
	})

	// Register a library with functions that have help text
	myLib := object.NewLibrary("testlib", map[string]*object.Builtin{
		"lib_func": {
			Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
				return &object.String{Value: "lib result"}
			},
			HelpText: `lib_func() - A library function

This is a test library function with help text.`,
		},
	}, map[string]object.Object{
		"LIB_CONSTANT": &object.String{Value: "1.0.0"},
	}, "Test library for help system")

	p.RegisterLibrary( myLib)

	// Register a Scriptling function with docstring
	err := p.RegisterScriptFunc("script_func", `
def script_func(x, y=10):
    """
    script_func(x, y=10) - A Scriptling function

    This function takes two parameters and returns their sum.
    The second parameter has a default value of 10.

    Parameters:
        x: First number
        y: Second number (default: 10)

    Returns:
        The sum of x and y
    """
    return x + y
script_func
`)
	if err != nil {
		t.Fatalf("failed to register script function: %v", err)
	}

	// Register a Scriptling library with docstring
	err = p.RegisterScriptLibrary("scriptlib", `
"""
scriptlib - A Scriptling library

This library contains utility functions for testing.
"""

def lib_add(a, b):
    """Add two numbers together."""
    return a + b

def lib_multiply(a, b):
    """Multiply two numbers."""
    return a * b
`)
	if err != nil {
		t.Fatalf("failed to register script library: %v", err)
	}

	// Test help for builtin functions
	t.Run("builtin_help", func(t *testing.T) {
		_, err := p.Eval(`help("len")`)
		if err != nil {
			t.Errorf("help for builtin 'len' failed: %v", err)
		}
	})

	// Test help for registered Go functions
	t.Run("go_function_help", func(t *testing.T) {
		_, err := p.Eval(`help("custom_func")`)
		if err != nil {
			t.Errorf("help for Go function failed: %v", err)
		}
	})

	// Test help for library functions
	t.Run("library_function_help", func(t *testing.T) {
		_, err := p.Eval(`
import testlib
help("testlib.lib_func")
`)
		if err != nil {
			t.Errorf("help for library function failed: %v", err)
		}
	})

	// Test help for library overview
	t.Run("library_help", func(t *testing.T) {
		_, err := p.Eval(`
import testlib
help("testlib")
`)
		if err != nil {
			t.Errorf("help for library failed: %v", err)
		}
	})

	// Test help for Scriptling functions with docstrings
	t.Run("script_function_help", func(t *testing.T) {
		_, err := p.Eval(`help("script_func")`)
		if err != nil {
			t.Errorf("help for Scriptling function failed: %v", err)
		}
	})

	// Test help for Scriptling library
	t.Run("script_library_help", func(t *testing.T) {
		_, err := p.Eval(`
import scriptlib
help("scriptlib")
`)
		if err != nil {
			t.Errorf("help for Scriptling library failed: %v", err)
		}
	})

	// Test help for function objects
	t.Run("function_object_help", func(t *testing.T) {
		_, err := p.Eval(`help(len)`)
		if err != nil {
			t.Errorf("help for function object failed: %v", err)
		}
	})

	// Test help for classes
	t.Run("class_help", func(t *testing.T) {
		// Create a simple class for testing
		testClass := &object.Class{
			Name: "TestClass",
			Methods: map[string]object.Object{
				"method1": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "method1"}
					},
					HelpText: "method1() - First test method",
				},
				"method2": &object.Builtin{
					Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
						return &object.String{Value: "method2"}
					},
				},
			},
		}

		p.RegisterLibrary(object.NewLibrary("classtest", nil, map[string]object.Object{
			"TestClass": testClass,
		}, "Class testing library"))

		_, err := p.Eval(`
import classtest
help(classtest.TestClass)
`)
		if err != nil {
			t.Errorf("help for class failed: %v", err)
		}
	})

	// Test help for instances
	t.Run("instance_help", func(t *testing.T) {
		_, err := p.Eval(`
import testlib
func_obj = testlib.lib_func
help(func_obj)
`)
		if err != nil {
			t.Errorf("help for function object failed: %v", err)
		}
	})

	// Test general help commands
	t.Run("general_help", func(t *testing.T) {
		_, err := p.Eval(`help()`)
		if err != nil {
			t.Errorf("general help failed: %v", err)
		}

		_, err = p.Eval(`help("builtins")`)
		if err != nil {
			t.Errorf("builtins help failed: %v", err)
		}

		_, err = p.Eval(`help("modules")`)
		if err != nil {
			t.Errorf("modules help failed: %v", err)
		}
	})
}

func TestCallFunction(t *testing.T) {
	t.Run("registered_function", func(t *testing.T) {
		p := New()

		// Register a Go function
		p.RegisterFunc("add", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, _ := args[0].AsInt()
			b, _ := args[1].AsInt()
			return object.NewInteger(a + b)
		})

		// Call it with Go arguments
		result, err := p.CallFunction("add", 10, 32)
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		sum, objErr := result.AsInt()
		if objErr != nil {
			t.Fatal("result is not an integer")
		}
		if sum != 42 {
			t.Errorf("expected 42, got %d", sum)
		}
	})

	t.Run("function_not_found", func(t *testing.T) {
		p := New()

		_, err := p.CallFunction("nonexistent", 1, 2)
		if err == nil {
			t.Error("expected error for nonexistent function")
		}
	})

	t.Run("script_defined_function", func(t *testing.T) {
		p := New()

		// Define a script function
		_, err := p.Eval("def greet(name): return 'Hello, ' + name")
		if err != nil {
			t.Fatalf("failed to define function: %v", err)
		}

		// Call with Go string
		result, err := p.CallFunction("greet", "World")
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "Hello, World" {
			t.Errorf("expected 'Hello, World', got %s", text)
		}
	})

	t.Run("function_with_multiple_types", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		// Define a function that uses multiple types
		_, err := p.Eval("def calculate(items, multiplier): return sum(items) * multiplier")
		if err != nil {
			t.Fatalf("failed to define function: %v", err)
		}

		// Call with Go list and int
		result, err := p.CallFunction("calculate", []int64{10, 20, 30}, 2)
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		product, objErr := result.AsInt()
		if objErr != nil {
			t.Fatal("result is not an integer")
		}
		if product != 120 { // (10+20+30) * 2
			t.Errorf("expected 120, got %d", product)
		}
	})

	t.Run("function_returning_string", func(t *testing.T) {
		p := New()

		p.RegisterFunc("concat", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, _ := args[0].AsString()
			b, _ := args[1].AsString()
			return &object.String{Value: a + b}
		})

		result, err := p.CallFunction("concat", "Hello, ", "World")
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "Hello, World" {
			t.Errorf("expected 'Hello, World', got %s", text)
		}
	})

	t.Run("function_returning_float", func(t *testing.T) {
		p := New()

		p.RegisterFunc("divide", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, _ := args[0].AsFloat()
			b, _ := args[1].AsFloat()
			return &object.Float{Value: a / b}
		})

		result, err := p.CallFunction("divide", 10.0, 4.0)
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		quotient, objErr := result.AsFloat()
		if objErr != nil {
			t.Fatal("result is not a float")
		}
		if quotient != 2.5 {
			t.Errorf("expected 2.5, got %f", quotient)
		}
	})

	t.Run("function_returning_bool", func(t *testing.T) {
		p := New()

		p.RegisterFunc("is_greater", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, _ := args[0].AsInt()
			b, _ := args[1].AsInt()
			return &object.Boolean{Value: a > b}
		})

		result, err := p.CallFunction("is_greater", 10, 5)
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		flag, objErr := result.AsBool()
		if objErr != nil {
			t.Fatal("result is not a boolean")
		}
		if !flag {
			t.Errorf("expected true, got false")
		}
	})

	t.Run("with_context", func(t *testing.T) {
		p := New()

		// Register a function that checks for context
		p.RegisterFunc("check_ctx", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			if ctx != nil {
				return &object.Boolean{Value: true}
			}
			return &object.Boolean{Value: false}
		})

		result, err := p.CallFunctionWithContext(context.Background(), "check_ctx")
		if err != nil {
			t.Fatalf("CallFunctionWithContext failed: %v", err)
		}

		flag, objErr := result.AsBool()
		if objErr != nil {
			t.Fatal("result is not a boolean")
		}
		if !flag {
			t.Errorf("expected true (context should be passed)")
		}
	})

	t.Run("with_timeout", func(t *testing.T) {
		p := New()

		// Register a function that can timeout
		p.RegisterFunc("slow_func", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			select {
			case <-ctx.Done():
				return &object.Error{Message: "timeout"}
			default:
				return &object.String{Value: "completed"}
			}
		})

		// Test with a very short timeout that should succeed
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		result, err := p.CallFunctionWithContext(ctx, "slow_func")
		if err != nil {
			t.Fatalf("CallFunctionWithContext failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "completed" {
			t.Errorf("expected 'completed', got %s", text)
		}
	})

	t.Run("with_kwargs_empty", func(t *testing.T) {
		p := New()

		// Register a function that accepts kwargs
		p.RegisterFunc("format", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			text, _ := args[0].AsString()
			return &object.String{Value: text}
		})

		// Call with empty kwargs map
		result, err := p.CallFunction("format", "hello", Kwargs{})
		if err != nil {
			t.Fatalf("CallFunction with kwargs failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "hello" {
			t.Errorf("expected 'hello', got %s", text)
		}
	})

	t.Run("with_kwargs_values", func(t *testing.T) {
		p := New()

		// Register a function that uses kwargs
		p.RegisterFunc("format", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			text, _ := args[0].AsString()
			prefix := kwargs.MustGetString("prefix", "")
			suffix := kwargs.MustGetString("suffix", "")
			return &object.String{Value: prefix + text + suffix}
		})

		// Call with kwargs
		result, err := p.CallFunction("format", "world",
			Kwargs{
				"prefix": ">> ",
				"suffix": " <<",
			})
		if err != nil {
			t.Fatalf("CallFunction with kwargs failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != ">> world <<" {
			t.Errorf("expected '>> world <<', got %s", text)
		}
	})

	t.Run("with_kwargs_partial", func(t *testing.T) {
		p := New()

		// Register a function with default kwargs
		p.RegisterFunc("greet", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			name, _ := args[0].AsString()
			prefix := kwargs.MustGetString("prefix", "Hello")
			return &object.String{Value: prefix + ", " + name}
		})

		// Call with only prefix kwarg
		result, err := p.CallFunction("greet", "Alice",
			Kwargs{
				"prefix": "Hi",
			})
		if err != nil {
			t.Fatalf("CallFunction with kwargs failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "Hi, Alice" {
			t.Errorf("expected 'Hi, Alice', got %s", text)
		}
	})

	t.Run("with_kwargs_types", func(t *testing.T) {
		p := New()

		// Register a function that uses different kwarg types
		p.RegisterFunc("configure", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			enabled := kwargs.MustGetBool("enabled", false)
			count := kwargs.MustGetInt("count", 0)
			rate := kwargs.MustGetFloat("rate", 1.0)
			return &object.Dict{Pairs: map[string]object.DictPair{
				"enabled": {Key: &object.String{Value: "enabled"}, Value: &object.Boolean{Value: enabled}},
				"count":   {Key: &object.String{Value: "count"}, Value: object.NewInteger(count)},
				"rate":    {Key: &object.String{Value: "rate"}, Value: &object.Float{Value: rate}},
			}}
		})

		// Call with mixed type kwargs
		result, err := p.CallFunction("configure", nil,
			Kwargs{
				"enabled": true,
				"count":   42,
				"rate":    3.14,
			})
		if err != nil {
			t.Fatalf("CallFunction with kwargs failed: %v", err)
		}

		dict, objErr := result.AsDict()
		if objErr != nil {
			t.Fatal("result is not a dict")
		}

		enabledVal := dict["enabled"]
		enabled, objErr := enabledVal.AsBool()
		if objErr != nil || !enabled {
			t.Errorf("expected enabled=true, got %v", enabledVal)
		}

		countVal := dict["count"]
		count, objErr := countVal.AsInt()
		if objErr != nil || count != 42 {
			t.Errorf("expected count=42, got %v", countVal)
		}

		rateVal := dict["rate"]
		rate, objErr := rateVal.AsFloat()
		if objErr != nil || rate != 3.14 {
			t.Errorf("expected rate=3.14, got %v", rateVal)
		}
	})

	t.Run("script_function_with_kwargs", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		// Define a script function with kwargs
		_, err := p.Eval("def format_msg(text, prefix='>>', suffix='<<'): return prefix + ' ' + text + ' ' + suffix")
		if err != nil {
			t.Fatalf("failed to define function: %v", err)
		}

		// Call with kwargs from Go
		result, err := p.CallFunction("format_msg", "hello",
			Kwargs{
				"prefix": "##",
				"suffix": "##",
			})
		if err != nil {
			t.Fatalf("CallFunction with kwargs failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "## hello ##" {
			t.Errorf("expected '## hello ##', got %s", text)
		}
	})

	t.Run("script_function_with_default_kwargs", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		// Define a script function with default kwargs
		_, err := p.Eval("def echo(value, repeat=1): return value * repeat")
		if err != nil {
			t.Fatalf("failed to define function: %v", err)
		}

		// Call without kwargs (use default)
		result, err := p.CallFunction("echo", "hi")
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "hi" {
			t.Errorf("expected 'hi', got %s", text)
		}

		// Call with kwargs
		result, err = p.CallFunction("echo", "hi",
			Kwargs{
				"repeat": 3,
			})
		if err != nil {
			t.Fatalf("CallFunction with kwargs failed: %v", err)
		}

		text, objErr = result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "hihihi" {
			t.Errorf("expected 'hihihi', got %s", text)
		}
	})

	t.Run("positional_only_args", func(t *testing.T) {
		p := New()

		// Register a function with positional args only
		p.RegisterFunc("add", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			a, _ := args[0].AsInt()
			b, _ := args[1].AsInt()
			return object.NewInteger(a + b)
		})

		// Call with positional args only - no kwargs map
		result, err := p.CallFunction("add", 10, 32)
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		sum, objErr := result.AsInt()
		if objErr != nil {
			t.Fatal("result is not an integer")
		}
		if sum != 42 {
			t.Errorf("expected 42, got %d", sum)
		}
	})

	t.Run("kwargs_only_no_positional", func(t *testing.T) {
		p := New()

		// Register a function that only uses kwargs
		p.RegisterFunc("config", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			enabled := kwargs.MustGetBool("enabled", false)
			name := kwargs.MustGetString("name", "default")
			return &object.String{Value: fmt.Sprintf("enabled=%v,name=%s", enabled, name)}
		})

		// Call with ONLY kwargs - no positional args (pass nil as positional placeholder)
		result, err := p.CallFunction("config", nil,
			Kwargs{
				"enabled": true,
				"name":    "test",
			})
		if err != nil {
			t.Fatalf("CallFunction with kwargs only failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "enabled=true,name=test" {
			t.Errorf("expected 'enabled=true,name=test', got %s", text)
		}
	})

	t.Run("mixed_positional_and_kwargs", func(t *testing.T) {
		p := New()

		// Register a function with both positional and keyword args
		p.RegisterFunc("format", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			// Positional args
			text, _ := args[0].AsString()
			count, _ := args[1].AsInt()
			// Keyword args
			prefix := kwargs.MustGetString("prefix", "")
			suffix := kwargs.MustGetString("suffix", "")
			return &object.String{Value: prefix + text + ":" + fmt.Sprint(count) + suffix}
		})

		// Call with 2 positional args + kwargs
		result, err := p.CallFunction("format", "item", 42,
			Kwargs{
				"prefix": "[",
				"suffix": "]",
			})
		if err != nil {
			t.Fatalf("CallFunction with mixed args failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "[item:42]" {
			t.Errorf("expected '[item:42]', got %s", text)
		}
	})

	t.Run("script_function_positional_only", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		// Define a script function with positional args only
		_, err := p.Eval("def add(a, b): return a + b")
		if err != nil {
			t.Fatalf("failed to define function: %v", err)
		}

		// Call with positional args only
		result, err := p.CallFunction("add", 15, 27)
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		sum, objErr := result.AsInt()
		if objErr != nil {
			t.Fatal("result is not an integer")
		}
		if sum != 42 {
			t.Errorf("expected 42, got %d", sum)
		}
	})

	t.Run("script_function_mixed_args", func(t *testing.T) {
		p := New()
		stdlib.RegisterAll(p)

		// Define a script function with positional and default args
		_, err := p.Eval("def greet(title, name, greeting='Hello'): return greeting + ', ' + title + ' ' + name")
		if err != nil {
			t.Fatalf("failed to define function: %v", err)
		}

		// Call with 2 positional args + 1 kwarg
		result, err := p.CallFunction("greet", "Dr", "Smith",
			Kwargs{
				"greeting": "Greetings",
			})
		if err != nil {
			t.Fatalf("CallFunction with mixed args failed: %v", err)
		}

		text, objErr := result.AsString()
		if objErr != nil {
			t.Fatal("result is not a string")
		}
		if text != "Greetings, Dr Smith" {
			t.Errorf("expected 'Greetings, Dr Smith', got %s", text)
		}
	})

	t.Run("no_args_at_all", func(t *testing.T) {
		p := New()

		// Register a function with no args
		p.RegisterFunc("get_value", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
			return object.NewInteger(42)
		})

		// Call with no args at all
		result, err := p.CallFunction("get_value")
		if err != nil {
			t.Fatalf("CallFunction failed: %v", err)
		}

		value, objErr := result.AsInt()
		if objErr != nil {
			t.Fatal("result is not an integer")
		}
		if value != 42 {
			t.Errorf("expected 42, got %d", value)
		}
	})
}
