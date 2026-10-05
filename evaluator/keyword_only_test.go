package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

// TestKeywordOnlyPositionalRejection: positional arguments must not fill
// keyword-only parameters. The compiled-call fast path for 1-3 argument
// calls once skipped the KeywordOnlyStart check, silently accepting
// g(1, 2) for `def g(a, *, b)` whenever the function had no defaults.
func TestKeywordOnlyPositionalRejection(t *testing.T) {
	for _, src := range []string{
		"def g(a, *, b):\n    return (a, b)\ng(1, 2)",
		"def g(a, /, *, b):\n    return (a, b)\ng(1, 2)",
		"def g(a, *, b, c):\n    return (a, b, c)\ng(1, 2, 3)",
	} {
		got := testEval(src)
		if got == nil || got.Type() != object.ERROR_OBJ {
			t.Errorf("%q should be rejected, got %v", src, got)
		}
	}

	ok := testEval("def g(a, /, *, b):\n    return (a, b)\ng(1, b=2)")
	if ok == nil || ok.Inspect() != "(1, 2)" {
		t.Errorf("g(1, b=2) = %v, want (1, 2)", ok)
	}
	ok = testEval("def g(a, *, b, c):\n    return (a, b, c)\ng(1, c=3, b=2)")
	if ok == nil || ok.Inspect() != "(1, 2, 3)" {
		t.Errorf("g(1, c=3, b=2) = %v, want (1, 2, 3)", ok)
	}
}
