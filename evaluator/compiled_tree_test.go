package evaluator

import (
	"context"
	"sync"
	"testing"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

// parseOnce parses src once so several evaluations can share one *ast.Program,
// the same sharing the parse cache and the compiled-closure cache produce.
func parseOnce(t *testing.T, src string) *ast.Program {
	t.Helper()
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if errs := p.Errors(); len(errs) != 0 {
		t.Fatalf("parse errors: %v", errs)
	}
	return program
}

func wantInt(t *testing.T, obj object.Object, want int64, what string) {
	t.Helper()
	intObj, ok := obj.(*object.Integer)
	if !ok {
		t.Fatalf("%s: got %T (%s), want Integer %d", what, obj, obj.Inspect(), want)
	}
	if intObj.IntValue() != want {
		t.Fatalf("%s: got %d, want %d", what, intObj.IntValue(), want)
	}
}

func wantString(t *testing.T, obj object.Object, want string, what string) {
	t.Helper()
	strObj, ok := obj.(*object.String)
	if !ok {
		t.Fatalf("%s: got %T (%s), want String %q", what, obj, obj.Inspect(), want)
	}
	if strObj.StringValue() != want {
		t.Fatalf("%s: got %q, want %q", what, strObj.StringValue(), want)
	}
}

// One compiled closure tree is cached per *ast.Program and shared by every
// evaluation of it, so location caches on its nodes (identifier slots, callee
// locations) must resolve correctly in each environment, not just the first.
func TestCompiledProgramSharedAcrossEnvironments(t *testing.T) {
	program := parseOnce(t, `
def g():
    return 3
g() + x * base
`)
	envs := make([]*object.Environment, 2)
	for i := range envs {
		envs[i] = object.NewEnvironment()
	}
	envs[0].Set("x", object.NewInteger(2))
	envs[0].Set("base", object.NewInteger(10))
	envs[1].Set("x", object.NewInteger(5))
	envs[1].Set("base", object.NewInteger(1))

	// Interleave rounds so caches populated by one environment are exercised
	// against the other.
	for round := 0; round < 3; round++ {
		wantInt(t, EvalWithContext(context.Background(), program, envs[0]), 23, "env0")
		wantInt(t, EvalWithContext(context.Background(), program, envs[1]), 8, "env1")
	}
}

// The same compiled program evaluated concurrently from independent
// environments (independent interpreter locks) must produce per-environment
// results; the compiled closures themselves are shared read-only. Meaningful
// under -race.
func TestCompiledProgramConcurrentIndependentEnvironments(t *testing.T) {
	program := parseOnce(t, `
def helper(v):
    return v * 2 + 1
acc = 0
for i in range(seed * 20):
    acc = acc + helper(i % 3)
d = {"a": 1, "b": 2}
pairs = 0
for k, v in d.items():
    pairs = pairs + v
evens = [n * n for n in range(6) if n % 2 == 0]
msg = "a" + "b" + "c" + str(pairs)
half = 0
while half < 4:
    half = half + 1
else:
    half = half + 10
try:
    bad = 1 / 0
except ZeroDivisionError:
    bad = -1
acc + pairs + len(evens) + len(msg) + half + bad
`)

	const n = 8
	results := make([]object.Object, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			env := object.NewEnvironment()
			env.Set("seed", object.NewInteger(int64(i+1)))
			results[i] = EvalWithContext(context.Background(), program, env)
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		m := (i + 1) * 20
		acc := 0
		for j := 0; j < m; j++ {
			acc += (j%3)*2 + 1
		}
		// pairs=3, len(evens)=3, len(msg)=4, half=14, bad=-1
		wantInt(t, results[i], int64(acc+23), "concurrent eval")
	}
}

// The `for x in range(...)` fast path is compiled whenever the call shape is
// static, but a runtime rebinding of `range` must divert to the generic call
// path — including on a loop node whose fast path already ran.
func TestCompiledRangeShadowing(t *testing.T) {
	obj := testEval(`
total = 0
for i in range(3):
    total = total + i
def two(n):
    return [10, 20]
range = two
for j in range(50):
    total = total + j
total
`)
	wantInt(t, obj, 33, "range shadowing")
}

// The per-arity call fast paths and the IntFast infix path are static
// decisions over a dynamic world: once the argument types drift, the call
// must fall back and surface the ordinary type error, not misbehave.
func TestCompiledCallFastPathTypeDrift(t *testing.T) {
	obj := testEval(`
def inc(a):
    return a + 1
x = inc(41)
y = inc("s")
`)
	if !object.IsError(obj) {
		t.Fatalf("inc(\"s\") should error, got %T (%s)", obj, obj.Inspect())
	}

	// Same drift inside a chain that IntFast claimed at parse time.
	obj = testEval(`
s = "x"
t = 1 + s
`)
	if !object.IsError(obj) {
		t.Fatalf("1 + \"x\" should error, got %T (%s)", obj, obj.Inspect())
	}
}

// The compile-time-flattened `+` chain must produce the same results and
// errors as the parenthesised forms.
func TestCompiledConcatChainEquivalence(t *testing.T) {
	wantString(t, testEval(`"a" + "b" + "c" + "d"`), "abcd", "flat chain")
	wantString(t, testEval(`"a" + ("b" + ("c" + "d"))`), "abcd", "grouped chain")
	wantInt(t, testEval(`1 + 2 + 3 * 2 + 4`), 13, "numeric chain")
	for _, src := range []string{
		`"x" + "y" + 1 + "z"`,
		`1 + 2 + 3 + "a"`,
	} {
		if obj := testEval(src); !object.IsError(obj) {
			t.Errorf("%s: want error, got %T (%s)", src, obj, obj.Inspect())
		}
	}
}

// Evaluating a bare statement (the non-Program path) compiles assignment
// targets lazily into the AST node's CompiledSlot; the same node evaluated
// against a second environment must reuse that cached form correctly.
func TestBareStatementAssignTargetCompiledOnce(t *testing.T) {
	program := parseOnce(t, `d["k"] = 99`)
	stmt := program.Statements[0]

	for round := 0; round < 2; round++ {
		env := object.NewEnvironment()
		d := &object.Dict{Pairs: make(map[string]object.DictPair)}
		env.Set("d", d)
		if obj := EvalWithContext(context.Background(), stmt, env); object.IsError(obj) {
			t.Fatalf("round %d: evaluation error: %s", round, obj.Inspect())
		}
		pair, ok := d.GetByString("k")
		if !ok {
			t.Fatalf("round %d: key k missing after assignment", round)
		}
		wantInt(t, pair.Value, 99, "assigned value")
	}
}

// A bare exception value flows through as an ordinary value and is not
// caught by except; only a raise unwinds.
func TestCompiledExceptionValueVsRaise(t *testing.T) {
	exc, ok := testEval(`ValueError("boom")`).(*object.Exception)
	if !ok {
		t.Fatalf("exception value: got %T", exc)
	}
	if exc.Raised {
		t.Fatal("a constructed exception value must not be marked raised")
	}

	exc, ok = testEval(`raise ValueError("boom")`).(*object.Exception)
	if !ok {
		t.Fatalf("raise: got %T", exc)
	}
	if !exc.Raised {
		t.Fatal("raise must mark the exception raised")
	}

	wantString(t, testEval(`
try:
    ValueError("x")
except ValueError:
    "caught"
"no"
`), "no", "bare exception value must not be caught")

	// A caught raise IS handled: the handler runs and binds normally. The
	// try statement's own value is None (v0.27.1 semantics: only control-flow
	// signals survive the handler), so read the binding afterwards.
	caught := testEval(`
try:
    raise ValueError("x")
except ValueError:
    r = "yes"
r
`)
	wantString(t, caught, "yes", "raised exception must be caught")
}

// An augmented assignment through a side-effecting index target evaluates the
// subscript expression once, as in Python: key() runs a single time, so
// len(log) is 1 and the result is 51.
func TestCompiledAugAssignSideEffectingIndex(t *testing.T) {
	wantInt(t, testEval(`
log = []
def key():
    log.append("k")
    return "a"
d = {"a": 0}
d[key()] += 5
d["a"] * 10 + len(log)
`), 51, "aug-assign via side-effecting index")
}
