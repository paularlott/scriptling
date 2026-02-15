package extlibs

import (
	"os"
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
)

func TestOSEnvironGet(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_OS_VAR", "test_value")
	defer os.Unsetenv("TEST_OS_VAR")

	p := scriptling.New()
	RegisterOSLibrary(p, nil)

	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name: "environ.get with existing var",
			code: `import os
result = os.environ.get("TEST_OS_VAR")
result`,
			expected: "test_value",
		},
		{
			name: "environ.get with default",
			code: `import os
result = os.environ.get("NONEXISTENT_VAR", "default")
result`,
			expected: "default",
		},
		{
			name: "environ.get without default returns empty",
			code: `import os
result = os.environ.get("NONEXISTENT_VAR", "")
result`,
			expected: "",
		},
		{
			name: "getenv with existing var",
			code: `import os
result = os.getenv("TEST_OS_VAR")
result`,
			expected: "test_value",
		},
		{
			name: "getenv with default",
			code: `import os
result = os.getenv("NONEXISTENT_VAR", "default")
result`,
			expected: "default",
		},
		{
			name: "environ direct access",
			code: `import os
result = os.environ["TEST_OS_VAR"]
result`,
			expected: "test_value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Eval(tt.code)
			if err != nil {
				t.Fatalf("Eval failed: %v", err)
			}

			str, ok := result.(*object.String)
			if !ok {
				t.Fatalf("Expected String, got %T", result)
			}

			if str.Value != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, str.Value)
			}
		})
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
