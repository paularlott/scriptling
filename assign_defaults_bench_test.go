package scriptling

import (
	"fmt"
	"testing"
)

// Benchmarks for the assignment and parameter-default paths: nested
// destructuring landed in assignToExpression/compileMultipleAssign, and
// defaults moved from call-time evaluation to definition-time resolution
// (Function.ResolvedDefaults). Call-heavy and def-creation shapes isolate the
// two directions of that trade.

// Plain name assignments in a loop: the compileAssign fast path.
func BenchmarkInterp_AssignPlain(b *testing.B) {
	benchScript(b, `
x = 0
for i in range(10000):
    x = i
    y = x
    z = y
`)
}

// Flat tuple unpacking per iteration: the MultipleAssignStatement path.
func BenchmarkInterp_UnpackFlat(b *testing.B) {
	benchScript(b, `
for i in range(10000):
    a, b = i, i + 1
`)
}

// Calls that omit defaulted parameters: defaultFor's resolved-value lookup.
func BenchmarkInterp_CallWithDefaults(b *testing.B) {
	benchScript(b, `
def f(a, b=10, c=20):
    return a + b + c
total = 0
for i in range(10000):
    total = f(i)
`)
}

// Calls filling a default by keyword: the keywords binding path.
func BenchmarkInterp_CallWithKwDefault(b *testing.B) {
	benchScript(b, `
def f(a, b=10, c=20):
    return a + b + c
total = 0
for i in range(10000):
    total = f(i, c=i)
`)
}

// Calls with every argument positional: the no-defaults fast path, guarding
// the funcParams changes against affecting default-free calls.
func BenchmarkInterp_CallExactArgs(b *testing.B) {
	benchScript(b, `
def f(a, b, c):
    return a + b + c
total = 0
for i in range(10000):
    total = f(i, 1, 2)
`)
}

// Def executed in a loop: resolveDefaults runs and allocates per execution.
func BenchmarkInterp_DefCreationLoop(b *testing.B) {
	benchScript(b, `
fns = []
for i in range(2000):
    def g(x, step=i):
        return x + step
    fns.append(g)
`)
}

// Lambda with a default created in a loop: resolution per creation.
func BenchmarkInterp_LambdaCreationLoop(b *testing.B) {
	benchScript(b, `
fns = []
for i in range(2000):
    fns.append(lambda x, n=i: x + n)
`)
}

// Parse time for an assignment-statement-heavy source: parseAssignValue and
// the parseExpressionStatement ASSIGN checks run per statement here.
func BenchmarkParse_AssignHeavy(b *testing.B) {
	script := ""
	for i := 0; i < 200; i++ {
		script += fmt.Sprintf("x%d = %d\ny%d, z%d = %d, %d\n", i, i, i, i, i, i+1)
	}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := parseProgramUncached(fmt.Sprintf("%s\n# v%d", script, i)); err != nil {
			b.Fatalf("unexpected parse error: %v", err)
		}
	}
}
