package stdlib_test

import (
	"context"
	"testing"

	"github.com/paularlott/scriptling/evaluator"
	"github.com/paularlott/scriptling/lexer"
	"github.com/paularlott/scriptling/object"
	"github.com/paularlott/scriptling/parser"
	"github.com/paularlott/scriptling/stdlib"
)

// TestRegexLibrarySurface exercises every function of the re library
// against its documented behaviour through real scripts: match/search/
// fullmatch anchoring, groups, findall and finditer, substitution with
// counts and backreferences, splitting, compile reuse, escape, flags, and
// invalid-pattern errors.
func TestRegexLibrarySurface(t *testing.T) {
	run := func(t *testing.T, src string) string {
		t.Helper()
		env := object.NewEnvironment()
		env.Set("re", stdlib.ReLibrary.GetDict())
		l := lexer.New(src)
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("parser errors: %v\n%s", p.Errors(), src)
		}
		result := evaluator.EvalWithContext(context.Background(), program, env)
		if result == nil {
			t.Fatal("nil result")
		}
		if isErr(result) {
			t.Fatalf("eval error: %s\n%s", result.Inspect(), src)
		}
		return result.Inspect()
	}

	t.Run("match anchors at the start", func(t *testing.T) {
		if got := run(t, `[re.match(r"\d+", "123abc").group(0), re.match(r"\d+", "abc123")]`); got != "[123, None]" {
			t.Fatalf("match anchoring: %s", got)
		}
	})
	t.Run("search finds anywhere", func(t *testing.T) {
		if got := run(t, `re.search(r"\d+", "abc123def").group(0)`); got != "123" {
			t.Fatalf("search: %s", got)
		}
	})
	t.Run("fullmatch spans the whole string", func(t *testing.T) {
		if got := run(t, `[re.fullmatch(r"\d+", "123").group(0), re.fullmatch(r"\d+", "123a")]`); got != "[123, None]" {
			t.Fatalf("fullmatch: %s", got)
		}
	})
	t.Run("groups", func(t *testing.T) {
		if got := run(t, `
m = re.match(r"(\w+)@(\w+)\.com", "ada@example.com")
[m.group(0), m.group(1), m.group(2)]
`); got != "[ada@example.com, ada, example]" {
			t.Fatalf("groups: %s", got)
		}
	})
	t.Run("findall", func(t *testing.T) {
		if got := run(t, `re.findall(r"\d+", "a1 b22 c333")`); got != "[1, 22, 333]" {
			t.Fatalf("findall: %s", got)
		}
	})
	t.Run("finditer iterates matches", func(t *testing.T) {
		if got := run(t, `
out = []
for m in re.finditer(r"\d+", "a1 b22"):
    out.append(m.group(0))
out
`); got != "[1, 22]" {
			t.Fatalf("finditer: %s", got)
		}
	})
	t.Run("sub replaces", func(t *testing.T) {
		if got := run(t, `re.sub(r"\d+", "N", "a1 b22")`); got != "aN bN" {
			t.Fatalf("sub: %s", got)
		}
	})
	t.Run("sub with a count limit", func(t *testing.T) {
		if got := run(t, `re.sub(r"\d+", "N", "a1 b22 c333", 2)`); got != "aN bN c333" {
			t.Fatalf("sub count: %s", got)
		}
	})
	t.Run("sub backreferences", func(t *testing.T) {
		if got := run(t, `re.sub(r"(\w+)@(\w+)", r"$2/$1", "ada@example")`); got != "example/ada" {
			t.Fatalf("sub backrefs: %s", got)
		}
	})
	t.Run("split", func(t *testing.T) {
		if got := run(t, `re.split(r"[,\s]+", "a, b,, c")`); got != "[a, b, c]" {
			t.Fatalf("split: %s", got)
		}
	})
	t.Run("compile reuse", func(t *testing.T) {
		if got := run(t, `
p = re.compile(r"\d+")
[p.match("123abc").group(0), re.findall(r"\d+", "a1b22")]
`); got != "[123, [1, 22]]" {
			t.Fatalf("compile: %s", got)
		}
	})
	t.Run("ignore-case flags", func(t *testing.T) {
		if got := run(t, `re.match("abc", "ABC", re.IGNORECASE).group(0)`); got != "ABC" {
			t.Fatalf("flags: %s", got)
		}
		if got := run(t, `re.compile("abc", re.I).match("ABC").group(0)`); got != "ABC" {
			t.Fatalf("compiled flags: %s", got)
		}
	})
	t.Run("escape", func(t *testing.T) {
		if got := run(t, `re.match(re.escape("a.b*c"), "a.b*c").group(0)`); got != "a.b*c" {
			t.Fatalf("escape: %s", got)
		}
	})
	t.Run("match spans", func(t *testing.T) {
		if got := run(t, `
m = re.search(r"\d+", "ab123cd")
[m.start(), m.end(), m.span()]
`); got != "[2, 5, (2, 5)]" {
			t.Fatalf("spans: %s", got)
		}
	})
	t.Run("invalid pattern errors", func(t *testing.T) {
		env := object.NewEnvironment()
		env.Set("re", stdlib.ReLibrary.GetDict())
		l := lexer.New("import re\nre.compile(\"(\")")
		p := parser.New(l)
		program := p.ParseProgram()
		if len(p.Errors()) != 0 {
			t.Fatalf("parser errors: %v", p.Errors())
		}
		if got := evaluator.EvalWithContext(context.Background(), program, env); !isErr(got) {
			t.Fatalf("expected an error for an invalid pattern, got %v", got)
		}
	})
}

func isErr(o object.Object) bool {
	if _, ok := o.(*object.Error); ok {
		return true
	}
	_, ok := o.(*object.Exception)
	return ok
}
