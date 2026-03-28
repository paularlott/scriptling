package evaluator

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

func TestStringPercentFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// %s - string
		{`"hello %s" % "world"`, "hello world"},
		{`"%s" % "test"`, "test"},
		{`"a%sb" % "X"`, "aXb"},

		// %d - integer
		{`"count: %d" % 42`, "count: 42"},
		{`"%d" % 0`, "0"},
		{`"%d" % -5`, "-5"},

		// %f - float
		{`"pi: %.2f" % 3.14159`, "pi: 3.14"},
		{`"val: %f" % 1.0`, "val: 1.000000"},

		// %.0f - zero decimal places (the original use case)
		{`"%.0f%%" % 75.5`, "76%"},

		// %x - hex
		{`"0x%x" % 255`, "0xff"},
		{`"0x%X" % 255`, "0xFF"},

		// %o - octal
		{`"%o" % 8`, "10"},

		// %% - literal percent (no value consumed)
		{`"100%%" % ()`, "100%"},
		{`"100%% done" % ()`, "100% done"},

		// Width specifiers
		{`"%5d" % 42`, "   42"},
		{`"%-5d" % 42`, "42   "},
		{`"%05d" % 42`, "00042"},

		// Float with int (coercion)
		{`"%f" % 5`, "5.000000"},

		// %d with float (truncation)
		{`"%d" % 3.7`, "3"},

		// Multiple values with tuple
		{`"%s is %d" % ("age", 25)`, "age is 25"},
		{`"%s %s" % ("hello", "world")`, "hello world"},
		{`"%d + %d = %d" % (1, 2, 3)`, "1 + 2 = 3"},

		// Mixed format specifiers with tuple
		{`"%s: %.1f%%" % ("CPU", 45.678)`, "CPU: 45.7%"},

		// %e - scientific notation
		{`"%e" % 1000.0`, "1.000000e+03"},

		// %g - general format
		{`"%g" % 100000.0`, "100000"},

		// %c - character from int
		{`"%c" % 65`, "A"},

		// %c - character from string
		{`"%c" % "Z"`, "Z"},

		// No format specifiers (empty tuple needed)
		{`"plain text" % ()`, "plain text"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(tt.input)
			result, ok := evaluated.(*object.Error)
			if ok {
				t.Fatalf("unexpected error: %s", result.Message)
			}
			str, ok := evaluated.(*object.String)
			if !ok {
				t.Fatalf("expected String, got %T: %v", evaluated, evaluated)
			}
			if str.Value != tt.expected {
				t.Errorf("got %q, want %q", str.Value, tt.expected)
			}
		})
	}
}

func TestStringPercentFormatErrors(t *testing.T) {
	tests := []struct {
		input       string
		errorSubstr string
	}{
		// Not enough arguments
		{`"%s %s" % "one"`, "not enough arguments"},
		// Too many arguments
		{`"%s" % ("a", "b")`, "not all arguments converted"},
		// %d with non-number
		{`"%d" % "hello"`, "a number is required"},
		// Incomplete format
		{`"test %" % 1`, "incomplete format"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			evaluated := testEval(tt.input)
			errObj, ok := evaluated.(*object.Error)
			if !ok {
				t.Fatalf("expected error containing %q, got %T: %v", tt.errorSubstr, evaluated, evaluated)
			}
			if !strings.Contains(errObj.Message, tt.errorSubstr) {
				t.Errorf("error message %q should contain %q", errObj.Message, tt.errorSubstr)
			}
		})
	}
}
