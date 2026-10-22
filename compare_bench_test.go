package scriptling

import "testing"

// Container comparison: equality, ordering, membership and searches over
// lists, tuples and dicts, and sorting and min/max over sequences, which all
// recurse through the comparison walkers.

func BenchmarkInterp_CompareEquality(b *testing.B) {
	benchScript(b, `
a = [[i, (i, str(i)), {"k": i}] for i in range(200)]
c = [[i, (i, str(i)), {"k": i}] for i in range(200)]
hits = 0
for r in range(20):
    if a == c:
        hits += 1
    if a[150] in c:
        hits += 1
    hits += c.count(a[199])
    hits += c.index(a[100])
`)
}

func BenchmarkInterp_CompareOrdering(b *testing.B) {
	benchScript(b, `
rows = [(i % 7, str(i % 13), [i % 3, i]) for i in range(300)]
for r in range(5):
    s = sorted(rows)
    lo = min(rows)
    hi = max(rows)
    lt = rows[10] < rows[20]
`)
}
