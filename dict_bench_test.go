package scriptling

import "testing"

// Small dict literals, lookups and iteration: the shapes payload-handling
// scripts are made of.
func BenchmarkInterp_SmallDicts(b *testing.B) {
	benchScript(b, `
total = 0
for i in range(2000):
    d = {"id": i, "name": "n", "tags": [], "ok": True}
    e = {"a": i, "b": i}
    total += d["id"] + e.get("b", 0)
    for k, v in e.items():
        total += v
    d["extra"] = e
    if "missing" in d:
        total += 1
`)
}

// Dicts grown key by key past the small-dict threshold, with deletes.
func BenchmarkInterp_GrowingDicts(b *testing.B) {
	benchScript(b, `
total = 0
for r in range(60):
    d = {}
    for i in range(120):
        d["key" + str(i)] = i
    for i in range(0, 120, 3):
        del d["key" + str(i)]
    for k in d:
        total += d[k]
    rows = [{"n": i, "sq": i * i} for i in range(60)]
    total += sum(row["sq"] for row in rows)
`)
}
