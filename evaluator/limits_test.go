package evaluator

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/conversion"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
)

// evalLimited evaluates src in a fresh interpreter whose guard is configured
// by configure, folding constants as the scriptling package does. It returns
// the result, or an error carrying the script error's text.
func evalLimited(t *testing.T, src string, configure func(g *object.ResourceGuard)) (object.Object, error) {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	ast.FoldConstants(program)
	env := object.NewEnvironment()
	g := object.NewResourceGuard()
	if configure != nil {
		configure(g)
	}
	env.SetResourceGuard(g)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result := EvalWithContext(ctx, program, env)
	if result == nil {
		t.Fatal("nil result")
	}
	if isRaised(result) || object.IsError(result) {
		return nil, fmt.Errorf("%s", result.Inspect())
	}
	return result, nil
}

func valueLimit(n int64) func(*object.ResourceGuard) {
	return func(g *object.ResourceGuard) { g.SetMaxValueBytes(n) }
}

// Every operation that sizes its result from script input is refused with a
// catchable MemoryError once the result would pass the value limit, and works
// as before below it.
func TestValueLimitOperations(t *testing.T) {
	const limit = 1000
	cases := []struct {
		name string
		ok   string // evaluates to a value within the limit
		want string // its Inspect()
		over string // must raise MemoryError
	}{
		// Variables keep the parser from folding the repetition into a
		// constant: a constant is part of the source, not a runtime value.
		{"str repeat", "n = 500\nlen(\"ab\" * n)", "1000", "n = 501\n\"ab\" * n"},
		{"int * str", "n = 500\nlen(n * \"ab\")", "1000", "n = 501\nn * \"ab\""},
		{"bytes repeat", `len(b"ab" * 500)`, "1000", `b"ab" * 501`},
		{"int * bytes", `len(500 * b"ab")`, "1000", `501 * b"ab"`},
		{"list repeat", `len([1] * 62)`, "62", `[1] * 63`},
		{"int * list", `len(62 * [1])`, "62", `63 * [1]`},
		{"tuple repeat", `len((1,) * 62)`, "62", `(1,) * 63`},
		{"int * tuple", `len(62 * (1,))`, "62", `63 * (1,)`},
		{"list *=", "x = [1]\nx *= 62\nlen(x)", "62", "x = [1]\nx *= 63"},
		{"str concat", "a = 'x' * 600\nb = 'y' * 400\nlen(a + b)", "1000", "a = 'x' * 600\nb = 'y' * 401\na + b"},
		{"str +=", "a = 'x' * 600\na += 'y' * 400\nlen(a)", "1000", "a = 'x' * 600\na += 'y' * 401"},
		{"bytes concat", "a = b'x' * 600\nlen(a + b'y' * 400)", "1000", "a = b'x' * 600\na + b'y' * 401"},
		{"list concat", "a = [1] * 31\nlen(a + [2] * 31)", "62", "a = [1] * 31\na + [2] * 32"},
		{"list + tuple", "a = [1] * 31\nlen(a + (2,) * 31)", "62", "a = [1] * 31\na + (2,) * 32"},
		{"tuple concat", "a = (1,) * 31\nlen(a + (2,) * 31)", "62", "a = (1,) * 31\na + (2,) * 32"},
		{"list +=", "a = [1] * 31\na += [2] * 31\nlen(a)", "62", "a = [1] * 31\na += [2] * 32"},
		{"append", "a = []\nfor i in range(62):\n    a.append(i)\nlen(a)", "62", "a = []\nfor i in range(1000):\n    a.append(i)"},
		{"builtin append", "a = []\nfor i in range(62):\n    append(a, i)\nlen(a)", "62", "a = []\nfor i in range(1000):\n    append(a, i)"},
		{"extend", "a = [1] * 31\na.extend([2] * 31)\nlen(a)", "62", "a = [1] * 31\na.extend([2] * 32)"},
		{"extend iterable", "class R:\n    def __iter__(self):\n        return iter(range(31))\na = [1] * 31\na.extend(R())\nlen(a)", "62", "class R:\n    def __iter__(self):\n        return iter(range(1000))\na = []\na.extend(R())"},
		{"insert", "a = [1] * 61\na.insert(0, 5)\nlen(a)", "62", "a = []\nfor i in range(1000):\n    a.insert(0, i)"},
		{"comprehension", "len([i for i in range(62)])", "62", "[i for i in range(1000)]"},
		{"filtered comprehension", "len([i for i in range(1000) if i < 62])", "62", "[i for i in range(1000) if i < 500]"},
		{"list(range)", "len(list(range(62)))", "62", "list(range(1000))"},
		{"list(str)", "len(list('x' * 62))", "62", "list('x' * 63)"},
		{"list(bytes)", "len(list(b'x' * 62))", "62", "list(b'x' * 63)"},
		{"tuple(iterator)", "len(tuple(iter(range(62))))", "62", "tuple(iter(range(1000)))"},
		{"center", `len("x".center(1000))`, "1000", `"x".center(1001)`},
		{"ljust", `len("x".ljust(1000, "-"))`, "1000", `"x".ljust(1001)`},
		{"rjust", `len("x".rjust(1000))`, "1000", `"x".rjust(1001)`},
		{"zfill", `len("5".zfill(1000))`, "1000", `"5".zfill(1001)`},
		{"zfill signed", `len("-5".zfill(1000))`, "1000", `"-5".zfill(1001)`},
		{"join", `len(",".join(["a" * 100] * 9))`, "908", `",".join(["a" * 100] * 10)`},
		{"bytes join", `len(b",".join([b"a" * 100] * 9))`, "908", `b",".join([b"a" * 100] * 10)`},
		{"replace", `len(("a" * 100).replace("a", "x" * 10))`, "1000", `("a" * 100).replace("a", "x" * 11)`},
		{"replace count", `len(("a" * 200).replace("a", "x" * 10, 50))`, "650", `("a" * 200).replace("a", "x" * 11, 90)`},
		{"replace empty", `len(("a" * 99).replace("", "x" * 9))`, "999", `("a" * 99).replace("", "x" * 10)`},
		{"expandtabs", `len(("\t" * 10).expandtabs(100))`, "1000", `("\t" * 10).expandtabs(101)`},
		{"f-string width", `len(f"{1:>1000}")`, "1000", `f"{1:>1001}"`},
		{"f-string precision", `len(f"{1.5:.900f}")`, "902", `f"{1.5:.1001f}"`},
		{"format()", `len(format("x", "<1000"))`, "1000", `format("x", "<1001")`},
		{"% width", `len("%1000s" % "x")`, "1000", `"%1001s" % "x"`},
		{"% precision", `len("%.900f" % 1.5)`, "902", `"%.1001f" % 1.5`},
		{"bytes(n)", `len(bytes(1000))`, "1000", `bytes(1001)`},
		{"dict setitem", "d = {}\nfor i in range(62):\n    d[i] = i\nlen(d)", "62", "d = {}\nfor i in range(63):\n    d[i] = i"},
		{"dict setitem existing key", "d = {}\nfor i in range(62):\n    d[i] = i\nd[0] = 9\nlen(d)", "62", "d = {}\nfor i in range(63):\n    d[i] = i"},
		{"dict comprehension", "len({i: i for i in range(62)})", "62", "{i: i for i in range(63)}"},
		{"dict update", "d = {i: i for i in range(31)}\nd.update({i: i for i in range(31, 62)})\nlen(d)", "62", "d = {i: i for i in range(31)}\nd.update({i: i for i in range(31, 63)})"},
		{"dict update pairs", "d = {}\nd.update([[i, i] for i in range(62)])\nlen(d)", "62", "d = {}\nd.update([[i, i] for i in range(63)])"},
		{"dict setdefault", "d = {}\nfor i in range(62):\n    d.setdefault(i, 0)\nlen(d)", "62", "d = {}\nfor i in range(63):\n    d.setdefault(i, 0)"},
		{"dict |=", "d = {i: i for i in range(31)}\nd |= {i: i for i in range(31, 62)}\nlen(d)", "62", "d = {i: i for i in range(31)}\nd |= {i: i for i in range(31, 63)}"},
		{"dict |", "a = {i: i for i in range(31)}\nlen(a | {i: i for i in range(31, 62)})", "62", "a = {i: i for i in range(31)}\na | {i: i for i in range(31, 63)}"},
		{"set add", "s = set()\nfor i in range(62):\n    s.add(i)\nlen(s)", "62", "s = set()\nfor i in range(63):\n    s.add(i)"},
		{"set comprehension", "len({i for i in range(62)})", "62", "{i for i in range(63)}"},
		{"set |=", "s = {i for i in range(31)}\ns |= {i for i in range(31, 62)}\nlen(s)", "62", "s = {i for i in range(31)}\ns |= {i for i in range(31, 63)}"},
		{"set |", "a = {i for i in range(31)}\nlen(a | {i for i in range(31, 62)})", "62", "a = {i for i in range(31)}\na | {i for i in range(31, 63)}"},
		{"dict() result", "len(dict([[i, i] for i in range(62)]))", "62", "dict([[i, i] for i in range(63)])"},
		{"starred list", "a = [0] * 31\nlen([*a, *a])", "62", "a = [0] * 32\n[*a, *a]"},
		{"starred tuple", "a = [0] * 31\nlen((*a, *a))", "62", "a = [0] * 32\n(*a, *a)"},
		{"*args from range", "def f(*a):\n    return len(a)\nf(*range(62))", "62", "def f(*a):\n    return len(a)\nf(*range(1000))"},
		{"*args from str", "def f(*a):\n    return len(a)\nf(*('x' * 62))", "62", "def f(*a):\n    return len(a)\nf(*('x' * 63))"},
		{"bytes.hex", `len((b"x" * 500).hex())`, "1000", `(b"x" * 501).hex()`},
		{"bytes.base64", `len((b"x" * 750).base64())`, "1000", `(b"x" * 751).base64()`},
		{"split sep", `len(("a," * 61).split(","))`, "62", `("a," * 62).split(",")`},
		{"split maxsplit", `len(("a," * 100).split(",", 61))`, "62", `("a," * 100).split(",", 62)`},
		{"split whitespace", `len(("a " * 62).split())`, "62", `("a " * 63).split()`},
		{"rsplit sep", `len(("a," * 61).rsplit(","))`, "62", `("a," * 62).rsplit(",")`},
		{"rsplit maxsplit", `len(("a," * 100).rsplit(",", 61))`, "62", `("a," * 100).rsplit(",", 62)`},
		{"rsplit whitespace", `len(("a " * 62).rsplit(None, 100))`, "62", `("a " * 63).rsplit(None, 100)`},
		{"splitlines", `len(("a\n" * 62).splitlines())`, "62", `("a\n" * 63).splitlines()`},
		{"bytes split sep", `len((b"a," * 61).split(b","))`, "62", `(b"a," * 62).split(b",")`},
		{"bytes split whitespace", `len((b"a " * 62).split())`, "62", `(b"a " * 63).split()`},
		{"translate expands", "t = str.maketrans({'a': 'x' * 10})\nlen(('a' * 100).translate(t))", "1000", "t = str.maketrans({'a': 'x' * 10})\n('a' * 101).translate(t)"},
		{"int.to_bytes", `len((1).to_bytes(1000))`, "1000", `(1).to_bytes(1001)`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := evalLimited(t, tc.ok, valueLimit(limit))
			if err != nil {
				t.Fatalf("within the limit: %v", err)
			}
			if result.Inspect() != tc.want {
				t.Fatalf("within the limit = %s, want %s", result.Inspect(), tc.want)
			}

			_, err = evalLimited(t, tc.over, valueLimit(limit))
			if err == nil {
				t.Fatal("over the limit: no error")
			}
			if !strings.Contains(err.Error(), "too large") && !strings.Contains(err.Error(), "limit") {
				t.Fatalf("over the limit: unexpected error %v", err)
			}

			// The refusal is a MemoryError the script can catch.
			catch := "try:\n" + indent(tc.over) + "\n    r = 'no error'\nexcept MemoryError as e:\n    r = 'caught'\nr"
			result, err = evalLimited(t, catch, valueLimit(limit))
			if err != nil {
				t.Fatalf("catching: %v", err)
			}
			if result.Inspect() != "caught" {
				t.Fatalf("except MemoryError did not catch: %s", result.Inspect())
			}
		})
	}
}

func indent(src string) string {
	lines := strings.Split(src, "\n")
	for i, l := range lines {
		lines[i] = "    " + l
	}
	return strings.Join(lines, "\n")
}

// MemoryError is a builtin exception class like the others.
func TestMemoryErrorBuiltin(t *testing.T) {
	result, err := evalLimited(t, `
caught = []
try:
    raise MemoryError("boom")
except MemoryError as e:
    caught.append(str(e))
try:
    raise MemoryError("again")
except Exception:
    caught.append("as Exception")
caught.append(isinstance(MemoryError("x"), MemoryError))
caught
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != `['boom', 'as Exception', True]` {
		t.Fatalf("got %s", got)
	}
}

// Without a value limit the operations are bounded only by the absolute
// ceiling that keeps size arithmetic from overflowing.
func TestUnlimitedValuesStillHaveACeiling(t *testing.T) {
	unlimited := valueLimit(0)
	for _, src := range []string{
		`"ab" * 4611686018427387904`,
		`[1, 2] * 4611686018427387904`,
		`b"ab" * 4611686018427387904`,
		`"x".center(100000000000000000)`,
		`f"{1:>100000000000000000000}"`,
		`"%100000000000000000000s" % "x"`,
		`f"{1.5:.100000000000000000000f}"`,
		`("\t" * 10).expandtabs(4611686018427387904)`,
		`("a" * 10).replace("a", "x" * 1000, 4611686018427387904) * 0`,
	} {
		_, err := evalLimited(t, src, unlimited)
		if err == nil && !strings.Contains(src, "* 0") {
			t.Errorf("%s: no error", src)
		}
	}
	// A large but reasonable value is allowed with no limit.
	result, err := evalLimited(t, `len("x" * (65 << 20))`, unlimited)
	if err != nil || result.Inspect() != fmt.Sprint(65<<20) {
		t.Fatalf("65 MiB string with no limit: %v %v", result, err)
	}
}

// With no limit set, nothing is refused; a default set for the process
// applies to environments without a guard.
func TestDefaultValueLimitApplies(t *testing.T) {
	if _, err := evalInEnv(t, "n = 65 << 20\nlen('x' * n)"); err != nil {
		t.Fatalf("65 MiB string with no limit set: %v", err)
	}
	old := object.DefaultMaxValueBytes()
	object.SetDefaultMaxValueBytes(1 << 20)
	defer object.SetDefaultMaxValueBytes(old)
	_, err := evalInEnv(t, "n = (1 << 20) + 1\n'x' * n")
	if err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("value over a 1 MiB default: %v", err)
	}
	if _, err := evalInEnv(t, "n = 1 << 20\nlen('x' * n)"); err != nil {
		t.Fatalf("1 MiB string under a 1 MiB default: %v", err)
	}
}

func TestExpandtabsEdgeCases(t *testing.T) {
	for src, want := range map[string]string{
		`"a\tb".expandtabs(0)`:           "ab",
		`"a\tb".expandtabs(-3)`:          "ab",
		`"a\tb".expandtabs(4)`:           "a   b",
		`"a\tb".expandtabs()`:            "a       b",
		`"ab".expandtabs(1000000000000)`: "ab",
	} {
		result, err := evalLimited(t, src, nil)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if result.Inspect() != want {
			t.Errorf("%s = %s, want %s", src, result.Inspect(), want)
		}
	}
}

func TestReplaceResults(t *testing.T) {
	for src, want := range map[string]string{
		`"aaa".replace("a", "bb")`:    "bbbbbb",
		`"aaa".replace("a", "bb", 2)`: "bbbba",
		`"aaa".replace("a", "")`:      "",
		`"ab".replace("", "-")`:       "-a-b-",
		`"abc".replace("x", "yy")`:    "abc",
		`"aaa".replace("a", "b", 0)`:  "aaa",
		`"aaa".replace("a", "b", -1)`: "bbb",
	} {
		result, err := evalLimited(t, src, valueLimit(1000))
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if result.Inspect() != want {
			t.Errorf("%s = %s, want %s", src, result.Inspect(), want)
		}
	}
}

// Slicing and indexing still give the right strings.
func TestSliceAndIndexResults(t *testing.T) {
	for src, want := range map[string]string{
		`("ab" * 1000)[1:4]`:                          "bab",
		`("ab" * 1000)[-3:]`:                          "bab",
		`("ab" * 1000)[1]`:                            "b",
		`("ab" * 1000)[-1]`:                           "b",
		`("é" * 1000)[1:3]`:                           "éé",
		`("é" * 1000)[2]`:                             "é",
		`(" x " * 200).strip()`:                       strings.TrimSpace(strings.Repeat(" x ", 200)),
		`("a,b" * 200).split(",")[1]`:                 "ba",
		`("k=v;" * 200).partition("=")[2][:3]`:        "v;k",
		`("pre" + "x" * 500).removeprefix("pre")[:2]`: "xx",
	} {
		result, err := evalLimited(t, src, func(g *object.ResourceGuard) { g.SetMemoryLimit(1 << 40) })
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if result.Inspect() != want {
			t.Errorf("%s = %q, want %q", src, result.Inspect(), want)
		}
	}
}

// List growth reports its backing arrays to the memory accounting.
func TestListGrowthIsAccounted(t *testing.T) {
	old := object.DefaultMemoryLimit()
	object.SetDefaultMemoryLimit(1 << 40) // accounting on, limit unreachable
	defer object.SetDefaultMemoryLimit(old)

	var g *object.ResourceGuard
	_, err := evalLimited(t, `
a = []
for i in range(200000):
    a.append(i)
b = [i for i in range(200000)]
c = a + b
d = [0] * 200000
`, func(guard *object.ResourceGuard) { g = guard; guard.SetMaxValueBytes(0) })
	if err != nil {
		t.Fatal(err)
	}
	// Four lists of about 200k slots (16 bytes each) were built.
	if got := g.Usage().AllocatedBytes; got < 4*200000*16 {
		t.Fatalf("list growth charged only %d bytes", got)
	}
}

func TestTupleLiteralValueLimit(t *testing.T) {
	if _, err := evalLimited(t, "len((1, 2, 3, 4))", valueLimit(64)); err != nil {
		t.Fatalf("four elements in 64 bytes: %v", err)
	}
	if _, err := evalLimited(t, "(1, 2, 3, 4, 5)", valueLimit(64)); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("five elements over 64 bytes: %v", err)
	}
}

// A literal's own size is checked before any of its elements is evaluated,
// so an oversized literal runs none of their side effects.
func TestLiteralCheckedBeforeEvaluation(t *testing.T) {
	for _, lit := range []string{
		"[f(), f(), f(), f(), f()]",
		"(f(), f(), f(), f(), f())",
		"{f(), f(), f(), f(), f()}",
		"{f(): 1, f(): 2, f(): 3, f(): 4, f(): 5}",
	} {
		src := "calls = []\ndef f():\n    calls.append(1)\n    return len(calls)\ntry:\n    x = " + lit + "\nexcept MemoryError:\n    pass\nlen(calls)"
		result, err := evalLimited(t, src, valueLimit(64))
		if err != nil {
			t.Fatalf("%s: %v", lit, err)
		}
		if result.Inspect() != "0" {
			t.Fatalf("%s: %s element expressions ran before the size was refused", lit, result.Inspect())
		}
	}
	// Within the limit the literal is built as usual.
	result, err := evalLimited(t, "calls = []\ndef f():\n    calls.append(1)\n    return len(calls)\nx = [f(), f(), f(), f()]\nx", valueLimit(64))
	if err != nil || result.Inspect() != "[1, 2, 3, 4]" {
		t.Fatalf("literal within the limit: %v %v", result, err)
	}
}

// Huge maxsplit values mean no limit: no overflow, no panic.
func TestSplitHugeMaxsplit(t *testing.T) {
	for src, want := range map[string]string{
		`"a,b,c".split(",", 9223372036854775807)`:   "['a', 'b', 'c']",
		`"a,b,c".rsplit(",", 9223372036854775807)`:  "['a', 'b', 'c']",
		`"a b c".split(None, 9223372036854775807)`:  "['a', 'b', 'c']",
		`"a b c".rsplit(None, 9223372036854775807)`: "['a', 'b', 'c']",
		`"a,b,c".rsplit(",", 1)`:                    "['a,b', 'c']",
		`b"a,b".split(b",", 9223372036854775807)`:   "[b'a', b'b']",
		`"".splitlines()`:                           "[]",
	} {
		result, err := evalLimited(t, src, valueLimit(8))
		if strings.HasPrefix(src, `""`) {
			if err != nil || result.Inspect() != want {
				t.Errorf("%s = %v, %v", src, result, err)
			}
			continue
		}
		result, err = evalLimited(t, src, nil)
		if err != nil {
			t.Errorf("%s: %v", src, err)
			continue
		}
		if result.Inspect() != want {
			t.Errorf("%s = %s, want %s", src, result.Inspect(), want)
		}
	}
}

// A split whose parts would not fit in the memory left is refused before
// splitting; a long string with few parts is not.
func TestSplitPeakMemory(t *testing.T) {
	mem := func(g *object.ResourceGuard) { g.SetMemoryLimit(1 << 20); g.SetMaxValueBytes(0) }
	_, err := evalLimited(t, `n = 400000
("a," * n).split(",")`, mem)
	if err == nil || !strings.Contains(err.Error(), "memory limit exceeded") {
		t.Fatalf("400,000 parts under 1 MiB: %v", err)
	}
	result, err := evalLimited(t, `n = 300000
len(("a" * n + ",b").split(","))`, mem)
	if err != nil || result.Inspect() != "2" {
		t.Fatalf("long string, two parts: %v %v", result, err)
	}
}

// evalWithTimeout evaluates src under a timeout of d, with no limits set,
// returning the result object (an error object if it was stopped).
func evalWithTimeout(t *testing.T, src string, d time.Duration) object.Object {
	t.Helper()
	l := lexer.New(src)
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v", p.Errors())
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	return EvalWithContext(ctx, program, object.NewEnvironment())
}

// Long-running loops that build values (spreading a huge iterator into call
// arguments, comprehensions over a huge range) stop on timeout instead of
// running on, and do not allocate their whole result up front.
func TestHugeIterationsStopOnTimeout(t *testing.T) {
	for _, src := range []string{
		"def f(*a):\n    return len(a)\nf(*iter(range(1000000000000)))",
		"[i for i in range(1000000000000)]",
		"{i: i for i in range(1000000000000)}",
		"[i for i in range(1000000000000) if False]",
	} {
		start := time.Now()
		result := evalWithTimeout(t, src, 200*time.Millisecond)
		if !object.IsError(result) || !strings.Contains(result.Inspect(), "timeout") {
			t.Fatalf("%s: %s", src, result.Inspect())
		}
		if time.Since(start) > 10*time.Second {
			t.Fatalf("%s: too slow to stop", src)
		}
	}
}

func TestCallDepthLimitError(t *testing.T) {
	cd := NewCallDepth(3)
	for i := range 3 {
		if !cd.Enter() {
			t.Fatalf("Enter %d failed", i)
		}
	}
	if cd.Enter() {
		t.Fatal("Enter past the depth limit succeeded")
	}
	if err := cd.LimitError(); !strings.Contains(err.Message, "call depth exceeded") {
		t.Fatalf("depth error = %s", err.Message)
	}
	if cd.Depth() != 3 {
		t.Fatalf("LimitError did not undo the failed Enter: depth %d", cd.Depth())
	}
}

func TestEnsureCallDepth(t *testing.T) {
	ctx := EnsureCallDepth(context.Background())
	cd := GetCallDepthFromContext(ctx)
	if cd == nil || int(cd.max) != DefaultMaxCallDepth {
		t.Fatalf("tracker = %+v", cd)
	}
	if again := EnsureCallDepth(ctx); GetCallDepthFromContext(again) != cd {
		t.Fatal("EnsureCallDepth replaced an existing tracker")
	}
}

// A literal's starred and ** entries get slots like fixed ones: an oversized
// literal is refused before any of its expressions run, whatever the mix.
func TestStarredLiteralCheckedBeforeEvaluation(t *testing.T) {
	const n = 5000 // 80,000 bytes of slots, over a 64 KiB value limit
	def := "calls = []\ndef g():\n    calls.append(1)\n    return ()\ndef m():\n    calls.append(1)\n    return {}\n"
	for name, lit := range map[string]string{
		"list":  "[" + strings.Repeat("*g(), ", n) + "]",
		"tuple": "(" + strings.Repeat("*g(), ", n) + ")",
		"set":   "{" + strings.Repeat("*g(), ", n) + "}",
		"dict":  "{" + strings.Repeat("**m(), ", n) + "}",
	} {
		src := def + "try:\n    x = " + lit + "\nexcept MemoryError:\n    pass\nlen(calls)"
		result, err := evalLimited(t, src, valueLimit(64<<10))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if result.Inspect() != "0" {
			t.Fatalf("%s: %s expressions ran before the size was refused", name, result.Inspect())
		}
	}
}

// The slots a starred literal allocates count against the memory limit.
func TestStarredLiteralSlotsCharged(t *testing.T) {
	src := "x = [" + strings.Repeat("*(), ", 5000) + "]"
	_, err := evalLimited(t, src, func(g *object.ResourceGuard) { g.SetMemoryLimit(64 << 10); g.SetMaxValueBytes(0) })
	if err == nil || !strings.Contains(err.Error(), "memory limit exceeded") {
		t.Fatalf("80 KB of literal slots under a 64 KiB memory limit: %v", err)
	}
}

// A bytes split copies every part out of the source, so a split into many
// parts that would not fit in the memory left is refused before splitting.
func TestBytesSplitPeakMemory(t *testing.T) {
	mem := func(g *object.ResourceGuard) { g.SetMemoryLimit(1 << 20); g.SetMaxValueBytes(0) }
	_, err := evalLimited(t, "n = 200000\n(b\"a,\" * n).split(b\",\")", mem)
	if err == nil || !strings.Contains(err.Error(), "memory limit exceeded") {
		t.Fatalf("200,000 parts under 1 MiB: %v", err)
	}
	_, err = evalLimited(t, "n = 200000\n(b\"a \" * n).split()", mem)
	if err == nil || !strings.Contains(err.Error(), "memory limit exceeded") {
		t.Fatalf("200,000 whitespace parts under 1 MiB: %v", err)
	}
	result, err := evalLimited(t, "n = 300000\nlen((b\"a\" * n + b\",b\").split(b\",\"))", mem)
	if err != nil || result.Inspect() != "2" {
		t.Fatalf("long bytes, two parts: %v %v", result, err)
	}
}

// Operations that know their full footprint up front are refused before
// allocating it, so the interpreter's peak never passes the memory limit.
func TestFootprintAdmittedBeforeAllocation(t *testing.T) {
	cases := map[string]string{
		"starred list":   "x = [" + strings.Repeat("*(), ", 5000) + "]",  // two 80,000-byte arrays
		"starred tuple":  "x = (" + strings.Repeat("*(), ", 5000) + ")",  // same
		"dict unpack":    "x = {" + strings.Repeat("**{}, ", 2000) + "}", // 2,000 presized entries at 88 bytes
		"str split":      "n = 100000\nx = (\"a,\" * n).split(\",\")",    // 200 KB copied out plus 100,000 parts
		"str splitlines": "n = 100000\nx = (\"a\\n\" * n).splitlines()",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			var g *object.ResourceGuard
			const limit = 120 << 10
			_, err := evalLimited(t, src, func(guard *object.ResourceGuard) {
				g = guard
				guard.SetMemoryLimit(limit)
				guard.SetMaxValueBytes(0)
			})
			if err == nil || !strings.Contains(err.Error(), "memory limit exceeded") {
				t.Fatalf("%v", err)
			}
			if peak := g.Usage().PeakMemoryBytes; peak > limit {
				t.Fatalf("peak %d bytes passed the %d byte limit before the refusal", peak, limit)
			}
		})
	}
}

// Deeply nested values are refused by the converters rather than exhausting
// the Go stack.
func TestDeeplyNestedConversion(t *testing.T) {
	result, err := evalLimited(t, `
x = []
for i in range(20000):
    x = [x]
x
`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if v := conversion.ToGo(result); v == nil {
		t.Fatal("plain ToGo returned nil for a deep value")
	}
	if _, errObj := conversion.ToGoLimited(result); errObj == nil || !strings.Contains(errObj.Message, "too deeply nested") {
		t.Fatalf("limited ToGo: %v", errObj)
	}
}

// A padding or format width is a single allocation request, so it is capped
// whatever the limits, with a MemoryError the script can catch.
func TestWidthCappedWithoutLimits(t *testing.T) {
	over := []string{
		`"%07000000000000d" % 1`,
		`"%7000000000000s" % "x"`,
		`"%.7000000000000f" % 1.0`,
		`"x".ljust(7000000000000)`,
		`"x".rjust(7000000000000)`,
		`"x".center(7000000000000)`,
		`"x".zfill(7000000000000)`,
		`"a\tb".expandtabs(7000000000000)`,
	}
	for _, src := range over {
		result, err := evalLimited(t, "try:\n    r = "+src+"\nexcept MemoryError as e:\n    r = 'caught: ' + str(e)\nr", nil)
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got := result.Inspect(); !strings.Contains(got, "caught:") || !strings.Contains(got, "too large") {
			t.Fatalf("%s: %s", src, got)
		}
	}
	result, err := evalLimited(t, `["%05d" % 1, "%7s" % "x", "%.3f" % 1.0, "x".ljust(3), "x".rjust(3), "x".center(3), "x".zfill(3), "a\tb".expandtabs(4)]`, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != `['00001', '      x', '1.000', 'x  ', '  x', ' x ', '00x', 'a   b']` {
		t.Fatalf("got %s", got)
	}
}

// A split on a one-byte separator can make one part per byte plus one, so
// the quick bound must not let a split over the limit skip the exact count:
// the split is refused before the list is built, not by the result check.
func TestSplitOneByteSeparatorCheckedBeforeSplitting(t *testing.T) {
	// 100,001 parts cost 1.6 MB at 16 bytes a slot; the limit admits 50,001.
	_, err := evalLimited(t, `("a" * 100000).split("a")`, valueLimit(900_000))
	if err == nil || !strings.Contains(err.Error(), "container too large") || strings.Contains(err.Error(), "result too large") {
		t.Fatalf("split into 100,001 parts under a 900 KB value limit: %v", err)
	}
	result, err := evalLimited(t, `[len(("a" * 1000).split("a")), len("a b  c".split()), ("x" * 100).split("x", 2)]`, valueLimit(900_000))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[1001, 3, ['', '', '"+strings.Repeat("x", 98)+"']]" {
		t.Fatalf("got %s", got)
	}
}

// A literal repetition folded when the script is parsed is checked against
// the value limit like the same expression with a variable multiplier.
func TestFoldedLiteralCheckedAgainstValueLimit(t *testing.T) {
	over := []string{
		`"ab" * 200000`,
		`b"ab" * 200000`,
		`f"{'y' * 300000}"`,
		`"x" * 150000 + "y" * 150000`,
	}
	for _, src := range over {
		result, err := evalLimited(t, "try:\n    r = len("+src+")\nexcept MemoryError:\n    r = 'caught'\nr", valueLimit(262144))
		if err != nil {
			t.Fatalf("%s: %v", src, err)
		}
		if got := result.Inspect(); got != "caught" {
			t.Fatalf("%s under a 256 KiB value limit: %s", src, got)
		}
	}
	result, err := evalLimited(t, `[len("ab" * 1000), len(b"ab" * 1000), "ab" * 3]`, valueLimit(262144))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "[2000, 2000, 'ababab']" {
		t.Fatalf("got %s", got)
	}
}

// Storing to an existing key never grows a dict or set, so it is not refused
// when one more entry would be.
func TestExistingKeyStoreNotRefused(t *testing.T) {
	result, err := evalLimited(t, `
d = {i: i for i in range(64)}
s = set(range(64))
out = []
d[1] = "updated"
s.add(1)
out.append(d[1])
out.append(len(s))
try:
    d[1000] = 1
    out.append("no error")
except MemoryError:
    out.append("dict refused")
try:
    s.add(1000)
    out.append("no error")
except MemoryError:
    out.append("set refused")
out
`, valueLimit(1024))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Inspect(); got != "['updated', 64, 'dict refused', 'set refused']" {
		t.Fatalf("got %s", got)
	}
}
