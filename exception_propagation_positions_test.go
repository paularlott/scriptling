package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// Every expression position that evaluates a sub-expression must propagate a
// raised exception to the nearest `except`, not swallow it as a value. These
// are the sites flagged in review after the initial isRaised conversion missed
// them (while/if/elif conditions, comprehensions, short-circuit and ternary
// operands, f-strings, dict literals, subscript-assign index, for-iterable,
// slice-read bounds).
func TestRaisePropagatesFromAllExpressionPositions(t *testing.T) {
	cases := map[string]string{
		"while condition":       "while boom():\n        break",
		"if condition":          "if boom():\n        pass",
		"elif condition":        "if False:\n        pass\n    elif boom():\n        pass",
		"list comprehension":    "x = [boom() for i in range(3)]",
		"dict comprehension":    "x = {i: boom() for i in range(3)}",
		"set comprehension":     "x = {boom() for i in range(3)}",
		"comprehension iterable": "x = [i for i in boom()]",
		"comprehension condition": "x = [i for i in range(3) if boom()]",
		"and operand":           "y = boom() and True",
		"or operand":            "y = False or boom()",
		"ternary condition":     "y = 1 if boom() else 2",
		"fstring interpolation": "s = f\"{boom()}\"",
		"dict literal key":      "d = {boom(): 1}",
		"dict literal value":    "d = {\"a\": boom()}",
		"list literal element":  "z = [1, boom(), 3]",
		"subscript assign index": "d = {}\n    d[boom()] = 1",
		"subscript assign value": "d = {}\n    d[\"k\"] = boom()",
		"for iterable":          "for x in boom():\n        pass",
		"slice read start":      "y = [1, 2, 3][boom():3]",
		"slice read end":        "y = [1, 2, 3][0:boom()]",
		"index operand":         "y = [1, 2, 3][boom()]",
		"range argument":        "for i in range(boom()):\n        pass",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			got := evalString(t, `
def boom():
    raise ValueError("bang")
caught = "no"
try:
    `+stmt+`
except ValueError:
    caught = "yes"
caught
`)
			if got != "yes" {
				t.Fatalf("%s: result = %q, want %q (raise was swallowed)", name, got, "yes")
			}
		})
	}
}

// The with statement must treat an exception VALUE produced in its body as a
// value, not a raise: __exit__ should see no exception and the block completes
// normally. (Inverted case from review.)
func TestWithBodyExceptionValueIsNotRaised(t *testing.T) {
	got := evalString(t, `
class CM:
    def __enter__(self):
        return self
    def __exit__(self, t, v, tb):
        return False

outcome = "start"
with CM() as cm:
    holder = ValueError("just-a-value")
outcome = "completed"
outcome
`)
	if got != "completed" {
		t.Fatalf("result = %q, want %q (exception value wrongly treated as raise)", got, "completed")
	}
}

// A with body that genuinely raises must still be seen by __exit__ / propagate.
func TestWithBodyRaisePropagates(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class CM:
    def __enter__(self):
        return self
    def __exit__(self, t, v, tb):
        return False

with CM() as cm:
    raise ValueError("real-raise")
`)
	if err == nil || !strings.Contains(err.Error(), "real-raise") {
		t.Fatalf("expected the raised exception to propagate, got err=%v", err)
	}
}

// A top-level expression that is an exception VALUE is a value, not an error,
// under the new semantics (handleResult must honour Raised). (Inverted case.)
func TestTopLevelExceptionValueIsNotError(t *testing.T) {
	p := New()
	result, err := p.Eval(`ValueError("just-a-value")`)
	if err != nil {
		t.Fatalf("a constructed exception value must not be an error, got: %v", err)
	}
	exc, ok := result.(*object.Exception)
	if !ok {
		t.Fatalf("expected an Exception value, got %T", result)
	}
	if exc.Raised {
		t.Fatalf("a constructed exception must not be marked Raised")
	}
	if exc.Message != "just-a-value" {
		t.Fatalf("message = %q, want %q", exc.Message, "just-a-value")
	}
}

// A scriptling function returning an exception instance hands back a value to
// the Go caller, not an error.
func TestFunctionReturningExceptionValueIsNotError(t *testing.T) {
	p := New()
	if _, err := p.Eval(`
def make():
    return ValueError("payload")
`); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	result, err := p.CallFunction("make")
	if err != nil {
		t.Fatalf("returning an exception value must not be an error, got: %v", err)
	}
	if exc, ok := result.(*object.Exception); !ok || exc.Message != "payload" {
		t.Fatalf("expected exception value 'payload', got %T %v", result, result)
	}
}

// Container reads for a missing element now raise Python-style, catchable
// exceptions instead of returning null.
func TestOutOfRangeReadsRaise(t *testing.T) {
	cases := []struct {
		name   string
		expr   string
		excYes string // except clause
	}{
		{"list index", "[1, 2][5]", "IndexError"},
		{"list negative index", "[1, 2][-5]", "IndexError"},
		{"tuple index", "(1, 2)[9]", "IndexError"},
		{"string index", `"hi"[9]`, "IndexError"},
		{"dict missing key", `{"a": 1}["missing"]`, "KeyError"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evalString(t, `
caught = "no"
try:
    _ = `+tc.expr+`
except `+tc.excYes+`:
    caught = "yes"
caught
`)
			if got != "yes" {
				t.Fatalf("%s: expected %s, result = %q", tc.name, tc.excYes, got)
			}
		})
	}
}

// A valid in-range read still returns the element (guard against over-reach).
func TestInRangeReadsStillWork(t *testing.T) {
	got := evalString(t, `
l = [10, 20, 30]
d = {"a": 1}
"%d|%d|%d" % (l[1], l[-1], d["a"])
`)
	if got != "20|30|1" {
		t.Fatalf("result = %q, want %q", got, "20|30|1")
	}
}
