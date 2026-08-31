package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestFinallyCannotReplaceProtectedExceptions pins that SystemExit keeps
// propagating even when the finally block raises: letting the replacement
// through would give an outer handler the chance to swallow an exit (or a
// PermissionError security refusal). The exception is planted as a variable
// and re-raised, which raise treats as an existing Exception verbatim.
func TestFinallyCannotReplaceProtectedExceptions(t *testing.T) {
	sl := New()
	if err := sl.SetObjectVar("exit_now", object.NewSystemExit(3, "exit for test")); err != nil {
		t.Fatalf("set var: %v", err)
	}

	_, err := sl.Eval(`
def f():
    try:
        raise exit_now
    finally:
        raise ValueError("replacement")

try:
    f()
except ValueError:
    pass   # swallowed the exit: exactly what must not happen
f()
`)
	if err == nil {
		t.Fatal("expected the SystemExit to propagate out")
	}
	if strings.Contains(err.Error(), "replacement") {
		t.Fatalf("finally's exception replaced the protected SystemExit: %v", err)
	}
	if !strings.Contains(err.Error(), "exit for test") {
		t.Fatalf("expected the SystemExit to survive, got: %v", err)
	}
}
