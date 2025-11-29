package scriptling

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/extlibs"
	"github.com/paularlott/scriptling/object"
)

func TestRegisterFunc(t *testing.T) {
	p := New()
	p.RegisterFunc("double", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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

	result, ok := p.GetVar("result")
	if !ok || result != int64(10) {
		t.Errorf("expected 10, got %v", result)
	}
}

func TestRegisterLibrary(t *testing.T) {
	p := New()
	myLib := object.NewLibrary(map[string]*object.Builtin{
		"greet": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				return &object.String{Value: "Hello!"}
			},
		},
	}, nil, "")
	p.RegisterLibrary("mylib", myLib)

	_, err := p.Eval(`
import mylib
msg = mylib.greet()
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	msg, ok := p.GetVar("msg")
	if !ok || msg != "Hello!" {
		t.Errorf("expected Hello!, got %v", msg)
	}
}

func TestRegisterLibraryWithClass(t *testing.T) {
	p := New()

	// Define a class in Go
	counterClass := &object.Class{
		Name: "Counter",
		Methods: map[string]object.Object{
			"__init__": &object.Builtin{
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
	myLib := object.NewLibrary(
		map[string]*object.Builtin{
			"helper": {
				Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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

	p.RegisterLibrary("counters", myLib)

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

	initial, ok := p.GetVar("initial")
	if !ok || initial != int64(10) {
		t.Errorf("expected initial=10, got %v", initial)
	}

	afterInc, ok := p.GetVar("after_inc")
	if !ok || afterInc != int64(11) {
		t.Errorf("expected after_inc=11, got %v", afterInc)
	}

	version, ok := p.GetVar("version")
	if !ok || version != "1.0.0" {
		t.Errorf("expected version=1.0.0, got %v", version)
	}

	helperResult, ok := p.GetVar("helper_result")
	if !ok || helperResult != "helper called" {
		t.Errorf("expected helper_result='helper called', got %v", helperResult)
	}
}

func TestImportBuiltin(t *testing.T) {
	p := New()
	_, err := p.Eval(`
import json
data = json.loads('{"key":"value"}')
result = data["key"]
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, ok := p.GetVar("result")
	if !ok || result != "value" {
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

	result, ok := p.GetVar("result")
	if !ok || result != int64(5) {
		t.Errorf("expected 5, got %v", result)
	}

	pi_value, ok := p.GetVar("pi_value")
	if !ok || pi_value != 3.14 {
		t.Errorf("expected 3.14, got %v", pi_value)
	}
}

func TestModuloOperator(t *testing.T) {
	p := New()
	_, err := p.Eval("result = 10 % 3")
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, ok := p.GetVar("result")
	if !ok || result != int64(1) {
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

	andResult, _ := p.GetVar("and_result")
	if andResult != false {
		t.Errorf("expected false, got %v", andResult)
	}

	orResult, _ := p.GetVar("or_result")
	if orResult != true {
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

	lte, _ := p.GetVar("lte")
	if lte != true {
		t.Errorf("expected true for <=, got %v", lte)
	}

	gte, _ := p.GetVar("gte")
	if gte != true {
		t.Errorf("expected true for >=, got %v", gte)
	}
}

func TestDotNotation(t *testing.T) {
	p := New()
	_, err := p.Eval(`
import json
data = json.loads('{"name":"Alice"}')
result = data["name"]
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	result, ok := p.GetVar("result")
	if !ok || result != "Alice" {
		t.Errorf("expected Alice, got %v", result)
	}
}

func TestHTTPLibrary(t *testing.T) {
	t.Skip("Skipping HTTP test due to unreliable external service")

	p := New()
	p.RegisterLibrary("requests", extlibs.RequestsLibrary)
	_, err := p.Eval(`
import requests
options = {"timeout": 10}
response = requests.get("https://httpbin.org/status/200", options)
print("Response:", response)
status = response.status_code
`)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	status, ok := p.GetVarAsInt("status")
	// Accept both 200 (success) and other status codes (service issues)
	if !ok || (status != int64(200) && status < 400) {
		t.Errorf("expected 200 or success status, got %v", status)
	}
}

func TestHelpSystem(t *testing.T) {
	p := New()

	// Register a Go function with help text
	p.RegisterFunc("custom_func", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		return &object.String{Value: "custom result"}
	})

	// Register a library with functions that have help text
	myLib := object.NewLibrary(map[string]*object.Builtin{
		"lib_func": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				return &object.String{Value: "lib result"}
			},
			HelpText: `lib_func() - A library function

This is a test library function with help text.`,
		},
	}, map[string]object.Object{
		"LIB_CONSTANT": &object.String{Value: "1.0.0"},
	}, "Test library for help system")

	p.RegisterLibrary("testlib", myLib)

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
					Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
						return &object.String{Value: "method1"}
					},
					HelpText: "method1() - First test method",
				},
				"method2": &object.Builtin{
					Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
						return &object.String{Value: "method2"}
					},
				},
			},
		}

		p.RegisterLibrary("classtest", object.NewLibrary(nil, map[string]object.Object{
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
