package extlibs

import (
	"testing"

	"github.com/paularlott/scriptling"
	"github.com/paularlott/scriptling/stdlib"
)

// Starting and awaiting background tasks, with no limits set.
func BenchmarkBackgroundTaskStart(b *testing.B) {
	ResetRuntime()
	SetSandboxFactory(nil)
	p := scriptling.New()
	stdlib.RegisterAll(p)
	RegisterRuntimeLibraryAll(p, nil)
	script := `
import scriptling.runtime as runtime
def work(n):
    return n + 1
for i in range(50):
    runtime.background("w", "work", i).get()
`
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
