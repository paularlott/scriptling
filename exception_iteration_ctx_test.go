package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// A user iterator whose __next__ raises a non-StopIteration exception must
// propagate it out of the for-loop (Python semantics), whether or not the loop
// body uses the yielded value. Previously this either looped forever or leaked
// the exception sideways.
func TestForUserIteratorNextRaisePropagates(t *testing.T) {
	prog := `
class BadIter:
    def __init__(self):
        self.n = 0
    def __iter__(self):
        return self
    def __next__(self):
        self.n = self.n + 1
        if self.n > 2:
            raise ValueError("next-boom")
        return self.n
`
	// body consumes the value
	got := evalString(t, prog+`
caught = "no"
try:
    total = 0
    for x in BadIter():
        total = total + x
except ValueError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("body-consumes: got %q, want yes", got)
	}

	// body ignores the value (would loop forever under the old bug)
	got = evalString(t, prog+`
caught = "no"
try:
    for x in BadIter():
        pass
except ValueError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("body-ignores: got %q, want yes", got)
	}
}

// An internal error (e.g. a NameError from a typo) in __next__ must surface,
// not silently end the loop.
func TestForUserIteratorNextInternalErrorSurfaces(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class TypoIter:
    def __iter__(self):
        return self
    def __next__(self):
        return undefined_variable_xyz

for x in TypoIter():
    pass
`)
	if err == nil {
		t.Fatal("expected the internal error in __next__ to surface")
	}
	if !strings.Contains(err.Error(), "undefined_variable_xyz") {
		t.Fatalf("error = %q, want it to mention the undefined name", err.Error())
	}
}

// A comprehension over a raising user iterator propagates too.
func TestComprehensionOverRaisingIteratorPropagates(t *testing.T) {
	got := evalString(t, `
class BadIter:
    def __init__(self):
        self.n = 0
    def __iter__(self):
        return self
    def __next__(self):
        self.n = self.n + 1
        if self.n > 2:
            raise ValueError("next-boom")
        return self.n

caught = "no"
try:
    y = [x for x in BadIter()]
except ValueError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("got %q, want yes", got)
	}
}

// A raise from __exit__ must propagate out of the with statement.
func TestWithExitRaisePropagates(t *testing.T) {
	p := New()
	_, err := p.Eval(`
class ExitRaises:
    def __enter__(self):
        return self
    def __exit__(self, t, v, tb):
        raise RuntimeError("exit-boom")

with ExitRaises() as e:
    pass
`)
	if err == nil || !strings.Contains(err.Error(), "exit-boom") {
		t.Fatalf("expected __exit__ raise to propagate, got err=%v", err)
	}
}

// If both the body and __exit__ raise, something must propagate (Python: the
// __exit__ exception wins) — never total silence.
func TestWithBodyAndExitRaiseNotSilenced(t *testing.T) {
	got := evalString(t, `
class ExitAlsoRaises:
    def __enter__(self):
        return self
    def __exit__(self, t, v, tb):
        raise RuntimeError("exit-wins")

which = "none"
try:
    with ExitAlsoRaises() as e:
        raise ValueError("body")
except RuntimeError:
    which = "exit"
except ValueError:
    which = "body"
which
`)
	if got != "exit" {
		t.Fatalf("got %q, want the __exit__ exception to win", got)
	}
}

// __exit__ returning truthy still suppresses a body raise.
func TestWithExitTruthySuppresses(t *testing.T) {
	got := evalString(t, `
class Suppress:
    def __enter__(self):
        return self
    def __exit__(self, t, v, tb):
        return True

outcome = "before"
try:
    with Suppress() as e:
        raise ValueError("suppressed")
    outcome = "completed"
except Exception:
    outcome = "leaked"
outcome
`)
	if got != "completed" {
		t.Fatalf("got %q, want the raise suppressed and block completed", got)
	}
}

// A raise in the with context-expression propagates instead of masking as a
// "requires __enter__/__exit__" error.
func TestWithContextExprRaisePropagates(t *testing.T) {
	got := evalString(t, `
def boom():
    raise ValueError("ctx-boom")

caught = "no"
try:
    with boom() as e:
        pass
except ValueError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("got %q, want yes", got)
	}
}

// A raise in the match subject propagates.
func TestMatchSubjectRaisePropagates(t *testing.T) {
	got := evalString(t, `
def boom():
    raise ValueError("subject-boom")

caught = "no"
try:
    match boom():
        case _:
            caught = "wrong"
except ValueError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("got %q, want yes", got)
	}
}

// bytes and float_array out-of-range reads raise IndexError, consistent with
// list/tuple/string/dict.
func TestBytesOutOfRangeRaises(t *testing.T) {
	got := evalString(t, `
data = bytes([97, 98, 99])
caught = "no"
try:
    _ = data[9]
except IndexError:
    caught = "yes"
caught
`)
	if got != "yes" {
		t.Fatalf("bytes: got %q, want yes", got)
	}
}

func TestFloatArrayOutOfRangeRaises(t *testing.T) {
	p := New()
	p.RegisterLibrary(stdlib.MathLibrary)
	result, err := p.Eval(`
import math
a = math.array([1.0, 2.0, 3.0])
caught = "no"
try:
    _ = a[9]
except IndexError:
    caught = "yes"
caught
`)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if got := result.Inspect(); got != "yes" {
		t.Fatalf("float_array: got %q, want yes", got)
	}
}
