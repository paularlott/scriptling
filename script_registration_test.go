package scriptling

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestRegisterScriptFunc(t *testing.T) {
	tests := []struct {
		name           string
		script         string
		funcName       string
		testScript     string
		expectedResult string
		expectError    bool
	}{
		{
			name:           "simple function",
			script:         "def add(a, b):\n    return a + b\nadd",
			funcName:       "add",
			testScript:     "result = add(2, 3)\nresult",
			expectedResult: "5",
			expectError:    false,
		},
		{
			name:           "function with default params",
			script:         "def greet(name, greeting=\"Hello\"):\n    return greeting + \", \" + name\ngreet",
			funcName:       "greet",
			testScript:     "result = greet(\"World\")\nresult",
			expectedResult: "Hello, World",
			expectError:    false,
		},
		{
			name:           "lambda function",
			script:         "lambda x: x * 2",
			funcName:       "double",
			testScript:     "result = double(5)\nresult",
			expectedResult: "10",
			expectError:    false,
		},
		{
			name:           "function with variadic args",
			script:         "def sum_all(*args):\n    total = 0\n    for x in args:\n        total = total + x\n    return total\nsum_all",
			funcName:       "sum_all",
			testScript:     "result = sum_all(1, 2, 3, 4, 5)\nresult",
			expectedResult: "15",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			err := p.RegisterScriptFunc(tt.funcName, tt.script)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			result, err := p.Eval(tt.testScript)
			if err != nil {
				t.Errorf("error evaluating test script: %v", err)
				return
			}

			if result.Inspect() != tt.expectedResult {
				t.Errorf("expected %s, got %s", tt.expectedResult, result.Inspect())
			}
		})
	}
}

func TestRegisterScriptLibrary(t *testing.T) {
	tests := []struct {
		name           string
		libName        string
		libScript      string
		testScript     string
		expectedResult string
		expectError    bool
	}{
		{
			name:    "simple library with functions",
			libName: "mylib",
			libScript: `
def add(a, b):
    return a + b

def multiply(a, b):
    return a * b

PI = 3.14159
`,
			testScript: `
import mylib
result = mylib.add(2, 3) + mylib.multiply(4, 5)
result
`,
			expectedResult: "25",
			expectError:    false,
		},
		{
			name:    "library with constants",
			libName: "constants",
			libScript: `
VERSION = "1.0.0"
MAX_SIZE = 100
ENABLED = True
`,
			testScript: `
import constants
result = constants.VERSION + " - " + str(constants.MAX_SIZE)
result
`,
			expectedResult: "1.0.0 - 100",
			expectError:    false,
		},
		{
			name:    "library with nested function calls",
			libName: "mathlib",
			libScript: `
def square(x):
    return x * x

def sum_of_squares(a, b):
    return square(a) + square(b)
`,
			testScript: `
import mathlib
result = mathlib.sum_of_squares(3, 4)
result
`,
			expectedResult: "25",
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			err := p.RegisterScriptLibrary(tt.libName, tt.libScript)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			result, err := p.Eval(tt.testScript)
			if err != nil {
				t.Errorf("error evaluating test script: %v", err)
				return
			}

			if result.Inspect() != tt.expectedResult {
				t.Errorf("expected %s, got %s", tt.expectedResult, result.Inspect())
			}
		})
	}
}

func TestNestedScriptLibraryImports(t *testing.T) {
	p := New()

	// Register a base library
	err := p.RegisterScriptLibrary("base", `
def helper(x):
    return x * 2
`)
	if err != nil {
		t.Fatalf("failed to register base library: %v", err)
	}

	// Register a library that imports the base library
	err = p.RegisterScriptLibrary("advanced", `
import base

def process(x):
    return base.helper(x) + 10
`)
	if err != nil {
		t.Fatalf("failed to register advanced library: %v", err)
	}

	// Use the advanced library
	result, err := p.Eval(`
import advanced
result = advanced.process(5)
result
`)
	if err != nil {
		t.Fatalf("error evaluating script: %v", err)
	}

	if result.Inspect() != "20" {
		t.Errorf("expected 20, got %s", result.Inspect())
	}
}

func TestScriptLibraryWithStandardLibrary(t *testing.T) {
	p := New()

	// Register a library that uses a standard library
	err := p.RegisterScriptLibrary("jsonutils", `
import json

def parse_and_get(json_str, key):
    data = json.loads(json_str)
    return data[key]
`)
	if err != nil {
		t.Fatalf("failed to register library: %v", err)
	}

	result, err := p.Eval(`
import jsonutils
result = jsonutils.parse_and_get('{"name": "Alice", "age": 30}', "name")
result
`)
	if err != nil {
		t.Fatalf("error evaluating script: %v", err)
	}

	if result.Inspect() != "Alice" {
		t.Errorf("expected Alice, got %s", result.Inspect())
	}
}

func TestRegisterScriptFuncWithGoFunc(t *testing.T) {
	p := New()

	// Register a Go function
	p.RegisterFunc("go_multiply", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
		if len(args) != 2 {
			return &object.Error{Message: "go_multiply requires 2 arguments"}
		}
		a, ok := args[0].(*object.Integer)
		if !ok {
			return &object.Error{Message: "first argument must be integer"}
		}
		b, ok := args[1].(*object.Integer)
		if !ok {
			return &object.Error{Message: "second argument must be integer"}
		}
		return &object.Integer{Value: a.Value * b.Value}
	})

	// Register a Scriptling function that uses the Go function
	err := p.RegisterScriptFunc("script_calc", `
def calc(x, y):
    return go_multiply(x, y) + 100
calc
`)
	if err != nil {
		t.Fatalf("failed to register script function: %v", err)
	}

	result, err := p.Eval("result = script_calc(5, 6)\nresult")
	if err != nil {
		t.Fatalf("error evaluating script: %v", err)
	}

	if result.Inspect() != "130" {
		t.Errorf("expected 130, got %s", result.Inspect())
	}
}

func TestRegisterScriptLibraryWithGoLibrary(t *testing.T) {
	p := New()

	// Register a Go library
	p.RegisterLibrary("golib", object.NewLibrary(map[string]*object.Builtin{
		"double": {
			Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
				if len(args) != 1 {
					return &object.Error{Message: "double requires 1 argument"}
				}
				i, ok := args[0].(*object.Integer)
				if !ok {
					return &object.Error{Message: "argument must be integer"}
				}
				return &object.Integer{Value: i.Value * 2}
			},
		},
	}, nil, ""))

	// Register a Scriptling library that uses the Go library
	err := p.RegisterScriptLibrary("scriptlib", `
import golib

def quad(x):
    return golib.double(golib.double(x))
`)
	if err != nil {
		t.Fatalf("failed to register script library: %v", err)
	}

	result, err := p.Eval(`
import scriptlib
result = scriptlib.quad(5)
result
`)
	if err != nil {
		t.Fatalf("error evaluating script: %v", err)
	}

	if result.Inspect() != "20" {
		t.Errorf("expected 20, got %s", result.Inspect())
	}
}

func TestLazyLoading(t *testing.T) {
	p := New()

	// Register a library with a syntax error
	// This should NOT fail because it's not evaluated yet
	err := p.RegisterScriptLibrary("broken", `
def broken_func():
    return @
`)
	if err != nil {
		t.Fatalf("failed to register broken library: %v", err)
	}

	// Register a working library
	err = p.RegisterScriptLibrary("working", `
def ok():
    return "ok"
`)
	if err != nil {
		t.Fatalf("failed to register working library: %v", err)
	}

	// Import working library - should succeed
	result, err := p.Eval(`
import working
working.ok()
`)
	if err != nil {
		t.Fatalf("error evaluating script: %v", err)
	}
	if result.Inspect() != "ok" {
		t.Errorf("expected ok, got %s", result.Inspect())
	}

	// Import broken library - should fail now
	_, err = p.Eval(`
import broken
`)
	if err == nil {
		t.Fatal("expected error importing broken library, got none")
	}

	// Verify caching - side effects should only happen once
	p = New()
	p.EnableOutputCapture()

	err = p.RegisterScriptLibrary("cached_lib", `
print("initializing library")
def func():
    return 1
`)
	if err != nil {
		t.Fatalf("failed to register cached lib: %v", err)
	}

	// First import
	_, err = p.Eval("import cached_lib")
	if err != nil {
		t.Fatalf("failed first import: %v", err)
	}

	// Check output
	output := p.GetOutput()
	if output != "initializing library\n" {
		t.Errorf("expected 'initializing library\\n', got '%s'", output)
	}

	// Clear output (simulated by checking if it appends or we just check content)
	// Actually GetOutput returns the buffer.

	// Second import
	_, err = p.Eval("import cached_lib")
	if err != nil {
		t.Fatalf("failed second import: %v", err)
	}

	// Output should be empty (no new print)
	output = p.GetOutput()
	if output != "" {
		t.Errorf("expected empty output, got '%s'", output)
	}
}
