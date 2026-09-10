package scriptling

import (
	"strings"
	"testing"
)

// evalString is a small helper: evaluates src and returns the Inspect() of the
// result, failing the test on an unexpected Eval error.
func evalString(t *testing.T, src string) string {
	t.Helper()
	p := New()
	result, err := p.Eval(src)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	return result.Inspect()
}

// A raised exception produced while evaluating a CALL ARGUMENT must propagate
// (unwind the stack) rather than be passed along as an argument value. This is
// the core waf-collector defect: `results.append(process_zone(...))` swallowed
// the raise and left errors uncounted.
func TestRaisedExceptionInCallArgumentPropagates(t *testing.T) {
	got := evalString(t, `
def boom():
    raise ValueError("network down")

errors = 0
results = []
try:
    results.append(boom())
except ValueError as e:
    errors = errors + 1
    results.append("caught:" + str(e))
"%d|%s" % (errors, results[0])
`)
	if got != "1|caught:network down" {
		t.Fatalf("result = %q, want %q", got, "1|caught:network down")
	}
}

// A CONSTRUCTED exception passed as an argument value must NOT propagate — it is
// an ordinary value. Regression guard so the fix above does not over-reach.
func TestConstructedExceptionArgumentIsValue(t *testing.T) {
	got := evalString(t, `
def kind(x):
    return x.__class__ if False else "got-value"
kind(ValueError("v"))
`)
	if got != "got-value" {
		t.Fatalf("result = %q, want %q", got, "got-value")
	}
}

// str(e) inside an except block must work: a caught exception is a value, not a
// propagating raise.
func TestCaughtExceptionIsUsableValue(t *testing.T) {
	got := evalString(t, `
try:
    raise ValueError("hello")
except ValueError as e:
    result = "msg=" + str(e)
result
`)
	if got != "msg=hello" {
		t.Fatalf("result = %q, want %q", got, "msg=hello")
	}
}

// Assigning a constructed exception to a variable binds it as a value (Python
// semantics); it does not raise.
func TestConstructedExceptionBindsAsValue(t *testing.T) {
	got := evalString(t, `
e = ValueError("stored")
str(e)
`)
	if got != "stored" {
		t.Fatalf("result = %q, want %q", got, "stored")
	}
}

// A raise inside various sub-expression positions must propagate to an
// enclosing except.
func TestRaisePropagatesFromSubExpressions(t *testing.T) {
	cases := map[string]string{
		"operand":      "x = boom() + 1",
		"list literal": "z = [1, boom(), 3]",
		"index":        "v = ({})[boom()]",
		"nested call":  "s = str(boom())",
		"method arg":   "[].append(boom())",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			got := evalString(t, `
def boom():
    raise ValueError("x")
caught = "no"
try:
    `+stmt+`
except ValueError:
    caught = "yes"
caught
`)
			if got != "yes" {
				t.Fatalf("%s: result = %q, want %q", name, got, "yes")
			}
		})
	}
}

// A bare `raise` inside an except must re-raise the caught exception.
func TestBareReRaise(t *testing.T) {
	p := New()
	_, err := p.Eval(`
try:
    raise ValueError("orig")
except ValueError:
    raise
`)
	if err == nil {
		t.Fatal("expected re-raised exception to surface as an error")
	}
	if !strings.Contains(err.Error(), "orig") {
		t.Fatalf("error = %q, want it to contain 'orig'", err.Error())
	}
}

// A raised exception on the right-hand side of a multiple-assignment
// (tuple unpacking) must propagate as that exception, not be swallowed and
// re-reported as "expected list or tuple, got EXCEPTION".
func TestMultipleAssignPropagatesRaisedException(t *testing.T) {
	got := evalString(t, `
def boom():
    raise ValueError("kaboom")

try:
    a, b, c = boom()
    result = "not caught"
except ValueError as e:
    result = "caught:" + str(e)
result
`)
	if got != "caught:kaboom" {
		t.Fatalf("result = %q, want %q", got, "caught:kaboom")
	}
}

// The same exception, when unhandled, must surface as the real error rather
// than the misleading tuple-unpacking type error.
func TestUnhandledMultipleAssignExceptionSurfacesRealError(t *testing.T) {
	p := New()

	_, err := p.Eval(`
def boom():
    raise ValueError("kaboom")

a, b, c = boom()
`)
	if err == nil {
		t.Fatal("expected unhandled exception to return an Eval error")
	}
	if strings.Contains(err.Error(), "expected list or tuple") {
		t.Fatalf("Eval error masked the real exception: %q", err.Error())
	}
	if !strings.Contains(err.Error(), "kaboom") {
		t.Fatalf("Eval error = %q, want the raised message 'kaboom'", err.Error())
	}
}
