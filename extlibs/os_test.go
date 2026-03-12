package extlibs

import (
	"os"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func TestOSGetenv(t *testing.T) {
	os.Setenv("TEST_OS_VAR", "test_value")
	defer os.Unsetenv("TEST_OS_VAR")
	os.Unsetenv("TEST_MISSING_VAR")

	p := scriptling.New()
	RegisterOSLibrary(p, nil)

	tests := []struct {
		name     string
		code     string
		check    func(t *testing.T, result object.Object)
	}{
		{
			name: "existing var returns value",
			code: `import os
os.getenv("TEST_OS_VAR")`,
			check: func(t *testing.T, result object.Object) {
				str, ok := result.(*object.String)
				if !ok {
					t.Fatalf("expected String, got %T", result)
				}
				if str.Value != "test_value" {
					t.Errorf("expected %q, got %q", "test_value", str.Value)
				}
			},
		},
		{
			name: "missing var without default returns None",
			code: `import os
os.getenv("TEST_MISSING_VAR")`,
			check: func(t *testing.T, result object.Object) {
				if _, ok := result.(*object.Null); !ok {
					t.Errorf("expected None/Null, got %T (%v)", result, result)
				}
			},
		},
		{
			name: "missing var with default returns default",
			code: `import os
os.getenv("TEST_MISSING_VAR", "fallback")`,
			check: func(t *testing.T, result object.Object) {
				str, ok := result.(*object.String)
				if !ok {
					t.Fatalf("expected String, got %T", result)
				}
				if str.Value != "fallback" {
					t.Errorf("expected %q, got %q", "fallback", str.Value)
				}
			},
		},
		{
			name: "existing var with default returns value not default",
			code: `import os
os.getenv("TEST_OS_VAR", "fallback")`,
			check: func(t *testing.T, result object.Object) {
				str, ok := result.(*object.String)
				if !ok {
					t.Fatalf("expected String, got %T", result)
				}
				if str.Value != "test_value" {
					t.Errorf("expected %q, got %q", "test_value", str.Value)
				}
			},
		},
		{
			name: "missing var returns None so 'if not' pattern works",
			code: `import os
val = os.getenv("TEST_MISSING_VAR")
if not val:
    val = "default_applied"
val`,
			check: func(t *testing.T, result object.Object) {
				str, ok := result.(*object.String)
				if !ok {
					t.Fatalf("expected String, got %T", result)
				}
				if str.Value != "default_applied" {
					t.Errorf("expected %q, got %q", "default_applied", str.Value)
				}
			},
		},
		{
			name: "var set to empty string returns empty string not None",
			code: `import os
os.getenv("TEST_OS_VAR")`,
			check: func(t *testing.T, result object.Object) {
				// TEST_OS_VAR is set to "test_value", not empty — just confirm it's a String
				if _, ok := result.(*object.String); !ok {
					t.Errorf("expected String for set var, got %T", result)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Eval(tt.code)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}
			tt.check(t, result)
		})
	}
}

func TestOSGetenvEmptyStringVar(t *testing.T) {
	// Explicitly set a var to empty string — should return "" not None
	os.Setenv("TEST_EMPTY_VAR", "")
	defer os.Unsetenv("TEST_EMPTY_VAR")

	p := scriptling.New()
	RegisterOSLibrary(p, nil)

	result, err := p.Eval(`import os
os.getenv("TEST_EMPTY_VAR")`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	str, ok := result.(*object.String)
	if !ok {
		t.Fatalf("expected String for empty-string var, got %T", result)
	}
	if str.Value != "" {
		t.Errorf("expected empty string, got %q", str.Value)
	}
}

func TestOSEnvironIsDict(t *testing.T) {
	p := scriptling.New()
	RegisterOSLibrary(p, nil)

	// Test that os.environ behaves like a dict by using .get() method
	result, err := p.Eval(`import os
result = os.environ.get("PATH", "default")
len(result) > 0`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	boolean, ok := result.(*object.Boolean)
	if !ok {
		t.Fatalf("Expected Boolean, got %T", result)
	}

	if !boolean.Value {
		t.Error("Expected os.environ.get() to work like a dict")
	}
}

func TestOSEnvironItems(t *testing.T) {
	os.Setenv("TEST_ITER_VAR", "iter_value")
	defer os.Unsetenv("TEST_ITER_VAR")

	p := scriptling.New()
	RegisterOSLibrary(p, nil)

	// Test that we can iterate over os.environ
	result, err := p.Eval(`import os
found = False
for key, value in os.environ.items():
    if key == "TEST_ITER_VAR" and value == "iter_value":
        found = True
        break
found`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	boolean, ok := result.(*object.Boolean)
	if !ok {
		t.Fatalf("Expected Boolean, got %T", result)
	}

	if !boolean.Value {
		t.Error("Expected to find TEST_ITER_VAR in os.environ.items()")
	}
}
