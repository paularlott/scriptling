package scriptling

import "testing"

// Loops that unpack (index, value), zipped and (key, value) pairs: each pair
// is one allocation.
func BenchmarkInterp_PairIteration(b *testing.B) {
	benchScript(b, `
names = ["a", "b", "c", "d", "e", "f", "g", "h"]
d = {"a": 1, "b": 2, "c": 3, "d": 4}
total = 0
for r in range(300):
    for i, name in enumerate(names):
        total += i
    for x, y in zip(names, names):
        total += 1
    for k, v in d.items():
        total += v
    pairs = list(d.items())
`)
}
