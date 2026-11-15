package scriptling_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/stdlib"
)

// Single operations and builtin calls that run long within one evaluator
// step must still stop at the call's deadline.

const (
	longCallTimeout = 300 * time.Millisecond
	longCallSlack   = 1500 * time.Millisecond
)

func expectLongCallDeadline(t *testing.T, script string) {
	t.Helper()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	ctx, cancel := context.WithTimeout(context.Background(), longCallTimeout)
	defer cancel()
	start := time.Now()
	result, err := p.EvalWithContext(ctx, script)
	elapsed := time.Since(start)
	msg := ""
	if err != nil {
		msg = err.Error()
	} else if e, ok := result.(*object.Error); ok {
		msg = e.Message
	} else {
		t.Fatalf("expected a timeout, script finished in %v with a %s", elapsed, result.Type())
	}
	if !strings.Contains(strings.ToLower(msg), "timeout") {
		t.Fatalf("expected a timeout error, got %q after %v", msg, elapsed)
	}
	if elapsed > longCallTimeout+longCallSlack {
		t.Fatalf("call ran %v past a %v deadline", elapsed-longCallTimeout, longCallTimeout)
	}
}

// Two equal values built from shared containers compare 2**60 elements in
// one == while holding a few hundred objects.
func TestSharedReferenceEqualityHonoursDeadline(t *testing.T) {
	expectLongCallDeadline(t, `x = [0]
y = [0]
for i in range(60):
    x = [x, x]
    y = [y, y]
x == y
`)
}

func TestSharedReferenceOrderingHonoursDeadline(t *testing.T) {
	expectLongCallDeadline(t, `x = (0,)
y = (0,)
for i in range(60):
    x = (x, x)
    y = (y, y)
x < y
`)
}

func TestSharedReferenceMembershipHonoursDeadline(t *testing.T) {
	expectLongCallDeadline(t, `x = [0]
y = [0]
for i in range(60):
    x = [x, x]
    y = [y, y]
x in [1, 2, y]
`)
}

// A sort whose comparisons are cheap Python calls (__lt__) runs no
// statement loop of its own.
func TestSortWithDunderComparisonsHonoursDeadline(t *testing.T) {
	expectLongCallDeadline(t, `class K:
    def __init__(self, v):
        self.v = v
    def __lt__(self, other):
        return self.v < other.v
items = [K(2), K(1)] * 5000000
sorted(items)
`)
}

func TestListSortHonoursDeadline(t *testing.T) {
	expectLongCallDeadline(t, `class K:
    def __init__(self, v):
        self.v = v
    def __lt__(self, other):
        return self.v < other.v
items = [K(2), K(1)] * 5000000
items.sort()
`)
}

// functools.reduce calls a lambda from Go for every element; a lambda has no
// statements, so only the callback check can stop it.
func TestReduceCallbacksHonourDeadline(t *testing.T) {
	expectLongCallDeadline(t, `import functools
items = [1] * 30000000
functools.reduce(lambda a, b: a + b, items)
`)
}

// A comparison that raised ends the sort without the remaining comparisons.
func TestSortStopsAtFirstRaise(t *testing.T) {
	p := scriptling.New()
	result, err := p.Eval(`calls = [0]
class K:
    def __lt__(self, other):
        calls[0] += 1
        raise ValueError("no")
try:
    sorted([K() for i in range(10000)])
except ValueError:
    pass
calls[0]
`)
	if err != nil {
		t.Fatal(err)
	}
	if n, _ := result.AsInt(); n != 1 {
		t.Fatalf("__lt__ called %d times after it raised, want 1", n)
	}
}
