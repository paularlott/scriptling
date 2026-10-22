package scriptling

import (
	"os"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// SCRIPTLING_BENCH_LIMITS=1 gives every interpreter the benchmarks create
// the limits below by default, so the whole suite can be compared with
// limits on and off from one binary. Only set it for benchmark runs.
func init() {
	if os.Getenv("SCRIPTLING_BENCH_LIMITS") == "1" {
		SetMemoryLimit(1 << 30)
		SetMaxValueBytes(64 << 20)
	}
}

// Benchmarks of the interpreter with every resource limit in force, to compare
// with the matching BenchmarkInterp_ ones. Turning memory accounting on is
// process-wide and permanent, so this file is named to run after the others,
// and the names stay out of the BenchmarkInterp_ set the PGO profile uses.

func benchLimitedScript(b *testing.B, script string) {
	b.Helper()
	p := New(WithMemoryLimit(1<<30), WithMaxValueBytes(64<<20))
	stdlib.RegisterAll(p)
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

func BenchmarkLimits_Strings(b *testing.B) {
	benchLimitedScript(b, `
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

func BenchmarkLimits_Collections(b *testing.B) {
	benchLimitedScript(b, `
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

func BenchmarkLimits_LoopArith(b *testing.B) {
	benchLimitedScript(b, `
total = 0
for i in range(20000):
    total = total + i * 2 - 1
`)
}

func BenchmarkLimits_Instances(b *testing.B) {
	benchLimitedScript(b, instanceBenchScript)
}
