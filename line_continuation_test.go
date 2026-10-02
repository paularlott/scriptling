package scriptling

import (
	"strings"
	"testing"
)

// Explicit line continuations evaluate exactly like the joined line, inside
// blocks and inside expressions, and an error on a continued statement is
// reported against the right line.
func TestLineContinuationEvaluates(t *testing.T) {
	p := New()
	result, err := p.Eval(`
def total(a, b, c):
    simple = (a > 0) and \
        (b > 0) and \
        c > 0
    if simple and \
            a < 100:
        return a + \
            b + c
    return 0

values = [total(1, 2, 3), total(0, 2, 3), total(200, 2, 3)]
values
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if got := result.Inspect(); got != "[6, 0, 0]" {
		t.Fatalf("result = %s, want [6, 0, 0]", got)
	}

	// The continuation line's indentation is not significant: the whole
	// statement belongs to the block it started in.
	result, err = p.Eval(`
out = []
for i in range(3):
    out.append(i * \
10)
out
`)
	if err != nil {
		t.Fatalf("eval failed: %v", err)
	}
	if got := result.Inspect(); got != "[0, 10, 20]" {
		t.Fatalf("result = %s, want [0, 10, 20]", got)
	}

	// An error inside a continued statement is reported against the
	// statement (its first line), as for any multi-line statement.
	_, err = p.Eval(`x = 1
y = x + \
    undefined_name
`)
	if err == nil || !strings.Contains(err.Error(), "undefined_name") || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("expected an undefined-name error on line 2, got: %v", err)
	}
}
