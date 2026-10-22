package scriptling

import "testing"

// Resuming generators: a for loop over one, next() calls, and a pipeline of
// generators each consuming the one before.
func BenchmarkInterp_Generators(b *testing.B) {
	benchScript(b, `
def count(n):
    i = 0
    while i < n:
        yield i
        i += 1

def double(src):
    for v in src:
        yield v * 2

total = 0
for v in count(2000):
    total += v
g = count(500)
for i in range(500):
    total += next(g)
total += sum(double(double(count(1000))))
`)
}
