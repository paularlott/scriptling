package stdlib_test

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

func benchStdlibScript(b *testing.B, script string) {
	b.Helper()
	p := scriptling.New()
	stdlib.RegisterAll(p)
	if _, err := p.Eval(script); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := p.Eval(script); err != nil {
			b.Fatal(err)
		}
	}
}

// A deque used as a FIFO queue and as a bounded window: right-end appends and
// left-end pops, the common case, which was already O(1).
func BenchmarkDequeQueue(b *testing.B) {
	benchStdlibScript(b, `
import collections
q = collections.deque()
w = collections.deque(maxlen=50)
for i in range(2000):
    q.append(i)
    w.append(i)
    if len(q) > 100:
        q.popleft()
total = sum(q) + sum(w) + len(collections.deque(range(500)))
`)
}

// Collecting iterators: extend from a generator-backed iterator, and the
// itertools functions that materialise one.
func BenchmarkCollectIterator(b *testing.B) {
	benchStdlibScript(b, `
import collections, itertools
d = collections.deque()
d.extend(x * 2 for x in range(2000))
c = list(itertools.chain(range(1000), range(1000)))
`)
}
