package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// map() must propagate a raised exception at the first raising element instead
// of storing the exception as a result element (delayed-propagation bug).
func TestMapPropagatesRaise(t *testing.T) {
	got := evalString(t, `
def boom(x):
    raise ValueError("map-boom")
caught = "no"
try:
    r = list(map(boom, [1, 2]))
except ValueError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("got %q, want yes (map swallowed the raise)", got)
	}
}

// filter() must propagate a predicate raise instead of coercing it to "keep".
func TestFilterPropagatesRaise(t *testing.T) {
	got := evalString(t, `
def boom(x):
    raise ValueError("filter-boom")
caught = "no"
try:
    r = list(filter(boom, [1, 2]))
except ValueError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("got %q, want yes (filter treated raise as truthy)", got)
	}
}

// A raising __bool__ must propagate through bool(), not, if/while, and the
// short-circuit operators — not be silently swallowed by truthiness.
func TestRaisingBoolPropagates(t *testing.T) {
	prelude := `
class B:
    def __bool__(self):
        raise ValueError("bool-boom")
`
	cases := map[string]string{
		"bool()":   "x = bool(B())",
		"not":      "x = not B()",
		"if":       "if B():\n        pass",
		"while":    "while B():\n        break",
		"and":      "x = B() and True",
		"or":       "x = B() or True",
		"ternary":  "x = 1 if B() else 2",
		"any":      "x = any([B()])",
		"all":      "x = all([B()])",
		"filter":   "x = list(filter(None, [B()]))",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			got := evalString(t, prelude+`
caught = "no"
try:
    `+stmt+`
except ValueError:
    caught = "yes"
caught
`)
			if got != "yes" {
				t.Fatalf("%s: got %q, want yes (__bool__ raise swallowed)", name, got)
			}
		})
	}
}

// functools.reduce must accept a lambda (and other callables), not only a def.
func TestReduceAcceptsLambda(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.FunctoolsLibrary)
	result, err := p.Eval(`
import functools
functools.reduce(lambda a, b: a + b, [1, 2, 3, 4])
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got := result.Inspect(); got != "10" {
		t.Fatalf("got %q, want 10", got)
	}
}

// A raise inside an except-type expression (e.g. a call in the tuple) must
// propagate rather than be silently ignored by structural name matching.
func TestExceptTypeExpressionRaisePropagates(t *testing.T) {
	got := evalString(t, `
def boom():
    raise ValueError("except-type-boom")
caught = "no"
try:
    try:
        raise KeyError("inner")
    except (boom(), KeyError):
        pass
except ValueError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("got %q, want yes (except-type raise swallowed)", got)
	}
}

// A raise from __init__ propagates out of instantiation (correctness), and a
// raising __init__ is catchable at the call site.
func TestInitRaisePropagates(t *testing.T) {
	got := evalString(t, `
class C:
    def __init__(self):
        raise ValueError("init-boom")
caught = "no"
try:
    c = C()
except ValueError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("got %q, want yes (__init__ raise swallowed)", got)
	}
}
