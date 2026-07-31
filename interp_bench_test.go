package scriptling

import (
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// Macro benchmarks used to track interpreter dispatch performance. Each one
// parses once (the parse cache makes repeat Eval calls hit the cached program)
// and then spends essentially all its time in the evaluator, so they isolate
// runtime dispatch rather than compile time.

func benchScript(b *testing.B, script string) {
	b.Helper()
	p := New()
	stdlib.RegisterAll(p)
	// Warm the parse cache so the measured loop is evaluation-dominated.
	if _, err := p.Eval(script); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := p.Eval(script); err != nil {
			b.Fatal(err)
		}
	}
}

// Deep recursion: stresses call setup, frame acquire/release, return values.
func BenchmarkInterp_CallHeavy(b *testing.B) {
	benchScript(b, `
def fib(n):
    if n <= 1:
        return n
    return fib(n-1) + fib(n-2)
result = fib(18)
`)
}

// Tight numeric loop: stresses the loop driver, identifier reads, int arithmetic.
func BenchmarkInterp_LoopArith(b *testing.B) {
	benchScript(b, `
total = 0
for i in range(20000):
    total = total + i * 2 - 1
`)
}

// Method dispatch and attribute access on instances.
func BenchmarkInterp_Methods(b *testing.B) {
	benchScript(b, `
class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y
    def dot(self, other):
        return self.x * other.x + self.y * other.y

a = Point(3, 4)
bb = Point(5, 6)
total = 0
for i in range(3000):
    total = total + a.dot(bb)
`)
}

// List and dict manipulation.
func BenchmarkInterp_Collections(b *testing.B) {
	benchScript(b, `
items = []
for i in range(2000):
    items.append(i)
d = {}
for i in range(2000):
    d[str(i)] = i
total = 0
for i in range(2000):
    total = total + items[i] + d[str(i)]
`)
}

// String building and methods.
func BenchmarkInterp_Strings(b *testing.B) {
	benchScript(b, `
parts = []
for i in range(1500):
    parts.append("item-" + str(i))
joined = ",".join(parts)
n = 0
for p in parts:
    if p.startswith("item-"):
        n = n + len(p.upper())
`)
}

// Mixed control flow: if/elif chains, while, break/continue, try.
func BenchmarkInterp_ControlFlow(b *testing.B) {
	benchScript(b, `
count = 0
i = 0
while i < 8000:
    i = i + 1
    if i % 3 == 0:
        count = count + 1
        continue
    elif i % 5 == 0:
        count = count + 2
    else:
        count = count + 3
    if i % 1000 == 0:
        try:
            raise ValueError("tick")
        except ValueError:
            count = count + 1
`)
}

// Long integer addition chains. These share the dispatch path with string
// concatenation chains, so they are worth tracking separately from LoopArith.
func BenchmarkInterp_IntAddChain(b *testing.B) {
	benchScript(b, `
a = 1
bb = 2
c = 3
d = 4
total = 0
for i in range(8000):
    total = total + a + bb + c + d + i
`)
}

// Comprehensions.
func BenchmarkInterp_Comprehension(b *testing.B) {
	benchScript(b, `
squares = [i * i for i in range(4000)]
evens = [x for x in squares if x % 2 == 0]
lookup = {str(i): i for i in range(2000)}
`)
}
