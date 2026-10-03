package scriptling

import (
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/stdlib"
)

// Collecting an infinite iterator (where Python would hang and exhaust
// memory) fails at once with an error that says how to bound it.
func TestCollectingInfiniteIteratorFails(t *testing.T) {
	for _, expr := range []string{
		"list(it.count())",
		"sorted(it.count())",
		"set(it.repeat(1))",
		"tuple(it.cycle('ab'))",
		"','.join(map(str, it.count()))",
		"sum(it.count())",
		"max(it.count())",
		"dict(zip(it.count(), it.count()))",
		"[x for x in it.count()]",
		"any(x > 3 for x in it.count())",
		"list(enumerate(it.cycle(iter([1]))))",
		"list(it.dropwhile(lambda x: x < 3, it.count()))",
		"print(*it.count())",
	} {
		t.Run(expr, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			done := make(chan error, 1)
			go func() {
				_, err := p.Eval("import itertools as it\n" + expr)
				done <- err
			}()
			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "cannot collect an infinite iterator") {
					t.Fatalf("got %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("hung collecting an infinite iterator")
			}
		})
	}
}
