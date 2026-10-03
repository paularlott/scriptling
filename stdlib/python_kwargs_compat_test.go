package stdlib_test

import (
	"context"
	"strings"
	"testing"

	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
	"github.com/paularlott/scriptling/stdlib"
)

// runCompat evaluates src with the re, itertools, random, textwrap, html and
// json libraries bound, failing on parse errors. It returns the Inspect() of
// the final expression (or of the error, when wantErr is true).
func runCompat(t *testing.T, src string, wantErr bool) string {
	t.Helper()
	env := object.NewEnvironment()
	env.Set("re", stdlib.ReLibrary.GetDict())
	env.Set("itertools", stdlib.ItertoolsLibrary.GetDict())
	env.Set("random", stdlib.RandomLibrary.GetDict())
	env.Set("textwrap", stdlib.TextwrapLibrary.GetDict())
	env.Set("html", stdlib.HTMLLibrary.GetDict())
	env.Set("json", stdlib.JSONLibrary.GetDict())
	p := parser.New(lexer.New(src))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatalf("parser errors: %v\n%s", p.Errors(), src)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5e9)
	defer cancel()
	result := evaluator.EvalWithContext(ctx, program, env)
	if result == nil {
		t.Fatalf("nil result\n%s", src)
	}
	failed := isErr(result) || result.Type() == object.EXCEPTION_OBJ
	if failed != wantErr {
		t.Fatalf("error=%v (want %v): %s\n%s", failed, wantErr, result.Inspect(), src)
	}
	return result.Inspect()
}

func checkCompat(t *testing.T, cases [][2]string) {
	t.Helper()
	for _, c := range cases {
		if got := runCompat(t, c[0], false); got != c[1] {
			t.Errorf("%s\n got: %s\nwant: %s", c[0], got, c[1])
		}
	}
}

// TestReFlagsKeyword: every re function accepts flags= as a keyword, as in
// Python (previously only the positional form was read).
func TestReFlagsKeyword(t *testing.T) {
	checkCompat(t, [][2]string{
		{`re.search("world", "HELLO WORLD", flags=re.I).group(0)`, "WORLD"},
		{`re.match("h", "Hi", flags=re.I).group(0)`, "H"},
		{`re.fullmatch("hi", "HI", flags=re.IGNORECASE).group(0)`, "HI"},
		{`re.findall("a", "AaA", flags=re.I)`, "['A', 'a', 'A']"},
		{`len(list(re.finditer("a", "AaA", flags=re.I)))`, "3"},
		{`re.sub("a", "x", "AaA", flags=re.I)`, "xxx"},
		{`re.sub("a", "x", "AaA", count=1, flags=re.I)`, "xaA"},
		{`re.subn("a", "x", "AaA", flags=re.I)`, "('xxx', 3)"},
		{`re.split("x", "aXbxc", flags=re.I)`, "['a', 'b', 'c']"},
		{`re.compile("a", flags=re.I).findall("AaA")`, "['A', 'a', 'A']"},
		{`re.findall("^b", "a\nb", flags=re.M)`, "['b']"},
		// positional form still works
		{`re.search("world", "HELLO WORLD", re.I).group(0)`, "WORLD"},
	})
	runCompat(t, `re.search("a", "A", re.I, flags=re.I)`, true)
}

// TestItertoolsKeywordArgs: groupby key=, accumulate func=/initial= and the
// keyword forms of permutations/combinations/batched r=/n=.
func TestItertoolsKeywordArgs(t *testing.T) {
	checkCompat(t, [][2]string{
		{`list(itertools.groupby(["aa", "ab", "ba"], key=lambda x: x[0]))`, "[('a', ['aa', 'ab']), ('b', ['ba'])]"},
		{`list(itertools.groupby([1, 1, 2], key=None))`, "[(1, [1, 1]), (2, [2])]"},
		{`list(itertools.accumulate([1, 2, 3, 4], func=lambda a, b: a * b))`, "[1, 2, 6, 24]"},
		{`list(itertools.accumulate([1, 2, 3, 4], lambda a, b: a * b))`, "[1, 2, 6, 24]"},
		{`list(itertools.accumulate([1, 2, 3], initial=100))`, "[100, 101, 103, 106]"},
		{`list(itertools.accumulate([], initial=5))`, "[5]"},
		{`list(itertools.accumulate([3, 1, 4], max, initial=2))`, "[2, 3, 3, 4]"},
		{`list(itertools.permutations([1, 2, 3], r=2))[:2]`, "[(1, 2), (1, 3)]"},
		{`list(itertools.permutations([1, 2], None))`, "[(1, 2), (2, 1)]"},
		{`list(itertools.combinations([1, 2, 3], r=2))`, "[(1, 2), (1, 3), (2, 3)]"},
		{`list(itertools.combinations_with_replacement("ab", r=2))`, "[('a', 'a'), ('a', 'b'), ('b', 'b')]"},
		{`list(itertools.batched([1, 2, 3, 4, 5], n=2))`, "[(1, 2), (3, 4), (5,)]"},
	})
	runCompat(t, `itertools.groupby([1], keyy=len)`, true)
	runCompat(t, `itertools.accumulate([1], lambda a, b: a, func=max)`, true)
}

// TestItertoolsInfiniteIteratorsAreLazy: cycle, count and repeat without a
// bound are lazy iterators, and consumers (enumerate, zip, islice,
// takewhile, compress) stop without materializing them.
func TestItertoolsInfiniteIteratorsAreLazy(t *testing.T) {
	checkCompat(t, [][2]string{
		{`list(itertools.islice(itertools.cycle([1, 2]), 5))`, "[1, 2, 1, 2, 1]"},
		{`out = []
for i, x in enumerate(itertools.cycle("ab")):
    if i > 3:
        break
    out.append((i, x))
out`, "[(0, 'a'), (1, 'b'), (2, 'a'), (3, 'b')]"},
		{`list(zip(range(3), itertools.cycle("xy")))`, "[(0, 'x'), (1, 'y'), (2, 'x')]"},
		{`list(itertools.islice(itertools.count(), 3))`, "[0, 1, 2]"},
		{`list(itertools.islice(itertools.count(10), 3))`, "[10, 11, 12]"},
		{`list(itertools.islice(itertools.count(start=1, step=2), 3))`, "[1, 3, 5]"},
		{`list(itertools.islice(itertools.count(0.5, step=0.5), 3))`, "[0.5, 1.0, 1.5]"},
		{`list(itertools.islice(itertools.repeat("x"), 3))`, "['x', 'x', 'x']"},
		{`list(zip(range(3), itertools.repeat(7)))`, "[(0, 7), (1, 7), (2, 7)]"},
		{`list(itertools.repeat(1, times=3))`, "[1, 1, 1]"},
		{`list(itertools.takewhile(lambda x: x < 4, itertools.count()))`, "[0, 1, 2, 3]"},
		{`list(itertools.compress("abcdef", itertools.cycle([1, 0])))`, "['a', 'c', 'e']"},
		{`list(itertools.islice(itertools.cycle(x for x in "ab"), 5))`, "['a', 'b', 'a', 'b', 'a']"},
		{`list(itertools.cycle([]))`, "[]"},
		// legacy finite forms are unchanged
		{`list(itertools.islice(itertools.count(0, 5), 3))`, "[0, 5, 10]"},
		{`itertools.cycle([1, 2], 2)`, "[1, 2, 1, 2]"},
		{`list(itertools.repeat("x", 2))`, "['x', 'x']"},
	})
}

// TestRandomChoicesCumWeights: cum_weights= is honoured (and exclusive with
// weights), and the other random functions take their keyword parameters.
func TestRandomChoicesCumWeights(t *testing.T) {
	checkCompat(t, [][2]string{
		{`random.seed(1)
set(random.choices("abc", cum_weights=[0, 0, 5], k=20))`, "{'c'}"},
		{`set(random.choices(["a", "b", "c"], cum_weights=(1, 1, 1), k=20))`, "{'a'}"},
		{`set(random.choices(["a", "b"], weights=[0, 1], k=10))`, "{'b'}"},
		{`len(random.sample(range(10), k=3))`, "3"},
		{`random.randint(a=4, b=4)`, "4"},
		{`0 <= random.triangular(0, 1, mode=None) <= 1`, "True"},
	})
	runCompat(t, `random.choices([1, 2], [1, 1], cum_weights=[1, 2])`, true)
	runCompat(t, `random.choices([1, 2], cum_weights=[1])`, true)
	runCompat(t, `random.choices([1, 2], bogus=1)`, true)
}

// TestTextwrapOptions compares wrap/fill/shorten/dedent/indent with output
// captured from CPython 3.14's textwrap.
func TestTextwrapOptions(t *testing.T) {
	const s = `"The quick brown fox jumps over the lazy dog. It was a well-known event.  Really! Supercalifragilisticexpialidocious words"`
	checkCompat(t, [][2]string{
		{`textwrap.wrap(` + s + `, 20, initial_indent="* ", subsequent_indent="  ")`,
			"['* The quick brown', '  fox jumps over the', '  lazy dog. It was a', '  well-known event.', '  Really! Supercalif', '  ragilisticexpialid', '  ocious words']"},
		{`textwrap.wrap(` + s + `, width=10, break_long_words=False)`,
			"['The quick', 'brown fox', 'jumps over', 'the lazy', 'dog. It', 'was a', 'well-known', 'event.', 'Really!', 'Supercalifragilisticexpialidocious', 'words']"},
		{`textwrap.wrap(` + s + `, width=15, break_on_hyphens=False)`,
			"['The quick brown', 'fox jumps over', 'the lazy dog.', 'It was a', 'well-known', 'event.  Really!', 'Supercalifragil', 'isticexpialidoc', 'ious words']"},
		{`textwrap.wrap(` + s + `, width=20, max_lines=2)`, "['The quick brown fox', 'jumps over the [...]']"},
		{`textwrap.wrap(` + s + `, width=20, max_lines=2, placeholder="...")`, "['The quick brown fox', 'jumps over the...']"},
		{`textwrap.wrap(` + s + `, width=12, drop_whitespace=False)`,
			"['The quick ', 'brown fox ', 'jumps over ', 'the lazy ', 'dog. It was ', 'a well-known', ' event.  ', 'Really! Supe', 'rcalifragili', 'sticexpialid', 'ocious words']"},
		{`textwrap.wrap(` + s + `, width=30, fix_sentence_endings=True)`,
			"['The quick brown fox jumps over', 'the lazy dog.  It was a well-', 'known event.  Really!  Superca', 'lifragilisticexpialidocious', 'words']"},
		{`textwrap.wrap(` + s + `, width=8)`,
			"['The', 'quick', 'brown', 'fox', 'jumps', 'over the', 'lazy', 'dog. It', 'was a', 'well-', 'known', 'event.', 'Really!', 'Supercal', 'ifragili', 'sticexpi', 'alidocio', 'us words']"},
		{`textwrap.fill("a\tb\tc  d", width=10)`, "a       b\nc  d"},
		{`textwrap.fill("a\tb", width=30, tabsize=4)`, "a   b"},
		{`textwrap.fill("a\tb\tc", width=30, expand_tabs=False, replace_whitespace=False)`, "a\tb\tc"},
		{`textwrap.fill("hello world", 5, initial_indent="> ")`, "> hel\nlo\nworld"},
		{`textwrap.shorten("Hello  world   this is a test", width=15, placeholder="...")`, "Hello world..."},
		{`textwrap.shorten("Hello World!", 11)`, "Hello [...]"},
		{`textwrap.dedent("    a\n      b\n   \n    c\n")`, "a\n  b\n\nc\n"},
		{`textwrap.dedent("\ta\n    b")`, "\ta\n    b"},
		{`textwrap.indent("a\n\nb\n", "> ")`, "> a\n\n> b\n"},
		{`textwrap.indent("a\n\nb\n", "> ", lambda l: True)`, "> a\n> \n> b\n"},
		{`textwrap.indent("a\nb", "> ", predicate=lambda l: l.startswith("b"))`, "a\n> b"},
	})
	runCompat(t, `textwrap.wrap("x", bogus=1)`, true)
	runCompat(t, `textwrap.fill("x", 10, width=10)`, true)
	runCompat(t, `textwrap.wrap("x", width=0)`, true)
}

// TestHTMLEscapeQuote: quote=False leaves quotes alone; the default output
// uses Python's entities (&quot; and &#x27;).
func TestHTMLEscapeQuote(t *testing.T) {
	checkCompat(t, [][2]string{
		{`html.escape("<a href=\"x\">'&", quote=False)`, `&lt;a href="x"&gt;'&amp;`},
		{`html.escape("\"'", False)`, `"'`},
		{`html.escape("<'\">")`, "&lt;&#x27;&quot;&gt;"},
		{`html.escape("<'\">", quote=True)`, "&lt;&#x27;&quot;&gt;"},
		{`html.unescape(html.escape("<'\"&>"))`, `<'"&>`},
	})
	runCompat(t, `html.escape("x", q=1)`, true)
}

// TestJSONDumpsTuples: tuples serialize as JSON arrays (also nested), and
// HTML-significant characters are not \u-escaped, matching Python.
func TestJSONDumpsTuples(t *testing.T) {
	checkCompat(t, [][2]string{
		{`json.dumps((1, 2))`, "[1,2]"},
		{`json.dumps({"a": (1, (2, "x"))})`, `{"a":[1,[2,"x"]]}`},
		{`json.dumps([(1,), ()])`, "[[1],[]]"},
		{`json.loads(json.dumps((1, 2)))`, "[1, 2]"},
		{`json.dumps("<a&b>")`, `"<a&b>"`},
	})
	if got := runCompat(t, `json.dumps({"a": (1, 2)}, indent=2)`, false); !strings.Contains(got, "[\n    1,\n    2\n  ]") {
		t.Errorf("indented tuple: %q", got)
	}
}
