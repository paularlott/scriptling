package evaluator

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

// Division by zero must be catchable as ZeroDivisionError, matching Python,
// while remaining an *object.Error so it still propagates through the
// evaluator's object.IsError checks and keeps its file/line annotation.

func runScript(t *testing.T, src string) (object.Object, *object.Environment) {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) > 0 {
		t.Fatalf("parse errors for %q: %v", src, errs)
	}
	env := object.NewEnvironment()
	return EvalWithContext(context.Background(), program, env), env
}

// resultOf runs src and returns the value bound to `result`.
func resultOf(t *testing.T, src string) string {
	t.Helper()
	out, env := runScript(t, src)
	if object.IsError(out) {
		t.Fatalf("unexpected error for %q: %s", src, out.Inspect())
	}
	val, ok := env.Get("result")
	if !ok {
		t.Fatalf("script did not bind `result`: %q", src)
	}
	return val.Inspect()
}

func TestZeroDivisionErrorIsCatchable(t *testing.T) {
	// Every operator that can divide by zero, with both literal and variable
	// operands (variables exercise the unboxed-integer path's fallback).
	exprs := []string{
		"1 / 0", "1 // 0", "1 % 0",
		"1.5 / 0.0", "1.5 // 0.0",
		"a / b", "a // b", "a % b",
		"a / 0", "10 // b",
		"(a + 1) // b", "a % (b + 0)",
	}
	for _, expr := range exprs {
		src := "a = 10\nb = 0\nresult = \"not run\"\ntry:\n    x = " + expr +
			"\nexcept ZeroDivisionError as e:\n    result = str(e)\n"
		if got := resultOf(t, src); !strings.Contains(got, "division by zero") {
			t.Errorf("%s: expected ZeroDivisionError to be caught, result = %q", expr, got)
		}
	}
}

func TestZeroDivisionErrorStillCaughtByBareException(t *testing.T) {
	// Backwards compatibility: code written before ZeroDivisionError was
	// classified catches these with `except Exception`.
	src := `
result = "not run"
try:
    x = 1 / 0
except Exception as e:
    result = str(e)
`
	if got := resultOf(t, src); !strings.Contains(got, "division by zero") {
		t.Errorf("expected `except Exception` to still catch division by zero, got %q", got)
	}
}

func TestZeroDivisionErrorMatchedInTupleAndOrdering(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{
			name: "tuple of types",
			src: "result = \"\"\ntry:\n    x = 1 / 0\n" +
				"except (KeyError, ZeroDivisionError):\n    result = \"tuple\"\n",
			want: "tuple",
		},
		{
			name: "earlier non-matching clause is skipped",
			src: "result = \"\"\ntry:\n    x = 1 / 0\n" +
				"except KeyError:\n    result = \"key\"\nexcept ZeroDivisionError:\n    result = \"zero\"\n",
			want: "zero",
		},
		{
			name: "propagates to an outer handler when inner does not match",
			src: "result = \"\"\ntry:\n    try:\n        x = 1 / 0\n" +
				"    except KeyError:\n        result = \"key\"\n" +
				"except ZeroDivisionError:\n    result = \"outer\"\n",
			want: "outer",
		},
		{
			name: "else block is skipped",
			src: "result = \"\"\ntry:\n    x = 1 / 0\n" +
				"except ZeroDivisionError:\n    result = \"caught\"\nelse:\n    result = \"else\"\n",
			want: "caught",
		},
		{
			name: "finally still runs",
			src: "result = \"\"\ntry:\n    x = 1 / 0\n" +
				"except ZeroDivisionError:\n    result = \"caught\"\nfinally:\n    result = result + \"+finally\"\n",
			want: "caught+finally",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := resultOf(t, c.src); got != c.want {
				t.Errorf("got %q want %q", got, c.want)
			}
		})
	}
}

func TestZeroDivisionErrorDoesNotSwallowOtherTypes(t *testing.T) {
	// `except ZeroDivisionError` must not catch unrelated exceptions.
	src := `
result = "not run"
try:
    raise ValueError("nope")
except ZeroDivisionError:
    result = "wrongly caught as ZeroDivisionError"
except ValueError as e:
    result = str(e)
`
	if got := resultOf(t, src); got != "nope" {
		t.Errorf("ValueError was misrouted, result = %q", got)
	}

	// And a ZeroDivisionError clause alone must let a ValueError propagate.
	out, _ := runScript(t, "try:\n    raise ValueError(\"boom\")\nexcept ZeroDivisionError:\n    x = 1\n")
	if !object.IsError(out) && out.Type() != object.EXCEPTION_OBJ {
		t.Errorf("expected the ValueError to propagate, got %s (%s)", out.Type(), out.Inspect())
	}
}

func TestZeroDivisionErrorUncaughtKeepsErrorShape(t *testing.T) {
	// Uncaught, it must remain an *object.Error carrying the original message and
	// line number — not become an "Uncaught exception: ..." wrapper. Scripts and
	// host code both depend on that shape.
	out, _ := runScript(t, "x = 1\ny = x / 0\n")
	err, ok := out.(*object.Error)
	if !ok {
		t.Fatalf("expected *object.Error, got %T (%s)", out, out.Inspect())
	}
	if err.Message != "division by zero" {
		t.Errorf("message = %q, want %q", err.Message, "division by zero")
	}
	if err.Line != 2 {
		t.Errorf("line = %d, want 2", err.Line)
	}
	if err.ExceptionType != object.ExceptionTypeZeroDivisionError {
		t.Errorf("ExceptionType = %q, want %q", err.ExceptionType, object.ExceptionTypeZeroDivisionError)
	}
}

func TestErrorExceptionTypeClassification(t *testing.T) {
	// An explicit tag wins; untagged errors fall back to message inference and
	// default to a plain Exception.
	cases := []struct {
		name string
		err  *object.Error
		want string
	}{
		{"explicit tag wins over message text", &object.Error{
			Message:       "type error: something",
			ExceptionType: object.ExceptionTypeZeroDivisionError,
		}, object.ExceptionTypeZeroDivisionError},
		{"inferred type error", &object.Error{Message: "type error: expected int, got str"}, object.ExceptionTypeTypeError},
		{"inferred name error", &object.Error{Message: "identifier not found: foo"}, object.ExceptionTypeNameError},
		{"inferred import error", &object.Error{Message: "import error: no such module"}, object.ExceptionTypeImportError},
		{"unclassified falls back to Exception", &object.Error{Message: "something went wrong"}, object.ExceptionTypeException},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := errorExceptionType(c.err); got != c.want {
				t.Errorf("errorExceptionType(%q) = %q, want %q", c.err.Message, got, c.want)
			}
		})
	}
}
