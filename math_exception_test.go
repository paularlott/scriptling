package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// math.fmod with a zero divisor must be catchable as ValueError, matching
// CPython (which reports "math domain error" as a ValueError there rather than a
// ZeroDivisionError). These tests go through the interpreter rather than calling
// the builtin directly, because what is being verified is except-clause matching.

// evalMathResult runs script with the math library registered and returns the
// string bound to `result`.
func evalMathResult(t *testing.T, script string) string {
	t.Helper()
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	if _, err := p.Eval(script); err != nil {
		t.Fatalf("unexpected error running script: %v\nscript:\n%s", err, script)
	}
	val, objErr := p.GetVar("result")
	if objErr != nil {
		t.Fatalf("result variable not found for script:\n%s", script)
	}
	s, ok := val.(string)
	if !ok {
		t.Fatalf("result was %T, want string, for script:\n%s", val, script)
	}
	return s
}

func TestMathFmodZeroDivisorCatchableAsValueError(t *testing.T) {
	cases := []struct{ name, script string }{
		{"int args", "import math\nresult = \"not run\"\ntry:\n    x = math.fmod(1, 0)\nexcept ValueError as e:\n    result = str(e)\n"},
		{"float args", "import math\nresult = \"not run\"\ntry:\n    x = math.fmod(5.5, 0.0)\nexcept ValueError as e:\n    result = str(e)\n"},
		{"mixed args", "import math\nresult = \"not run\"\ntry:\n    x = math.fmod(5.5, 0)\nexcept ValueError as e:\n    result = str(e)\n"},
		{"tuple of types", "import math\nresult = \"not run\"\ntry:\n    x = math.fmod(1, 0)\nexcept (KeyError, ValueError) as e:\n    result = str(e)\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := evalMathResult(t, c.script)
			if !strings.Contains(got, "division by zero") {
				t.Errorf("expected ValueError to be caught, result = %q", got)
			}
		})
	}
}

func TestMathFmodStillCaughtByBareException(t *testing.T) {
	// Backwards compatibility for code written before the error was classified.
	got := evalMathResult(t, "import math\nresult = \"not run\"\n"+
		"try:\n    x = math.fmod(1, 0)\nexcept Exception as e:\n    result = str(e)\n")
	if !strings.Contains(got, "division by zero") {
		t.Errorf("expected `except Exception` to still catch it, result = %q", got)
	}
}

func TestMathFmodIsNotAZeroDivisionError(t *testing.T) {
	// The two error kinds must not be conflated: fmod is a ValueError, while the
	// `/` operator is a ZeroDivisionError.
	got := evalMathResult(t, "import math\nresult = \"not run\"\n"+
		"try:\n    x = math.fmod(1, 0)\n"+
		"except ZeroDivisionError:\n    result = \"wrongly caught as ZeroDivisionError\"\n"+
		"except ValueError:\n    result = \"value error\"\n")
	if got != "value error" {
		t.Errorf("result = %q, want %q", got, "value error")
	}

	got = evalMathResult(t, "result = \"not run\"\n"+
		"try:\n    x = 1 / 0\n"+
		"except ValueError:\n    result = \"wrongly caught as ValueError\"\n"+
		"except ZeroDivisionError:\n    result = \"zero division\"\n")
	if got != "zero division" {
		t.Errorf("result = %q, want %q", got, "zero division")
	}
}

func TestMathFmodNormalOperationUnaffected(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	if _, err := p.Eval("import math\nresult = math.fmod(7.5, 2)"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	val, objErr := p.GetVar("result")
	if objErr != nil {
		t.Fatal("result variable not found")
	}
	if f, ok := val.(float64); !ok || f != 1.5 {
		t.Errorf("math.fmod(7.5, 2) = %v (%T), want 1.5", val, val)
	}
}

func TestMathFmodUncaughtStillErrors(t *testing.T) {
	// Uncaught, it must still surface as an error from Eval rather than being
	// silently swallowed.
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	_, err := p.Eval("import math\nx = math.fmod(1, 0)")
	if err == nil {
		t.Fatal("expected an error from an uncaught fmod zero divisor")
	}
	if !strings.Contains(err.Error(), "division by zero") {
		t.Errorf("error = %q, want it to mention division by zero", err.Error())
	}
}
