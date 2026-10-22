package tomllimit

import (
	"errors"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func deep(open, close string, n int) string {
	return "x = " + strings.Repeat(open, n) + "1" + strings.Repeat(close, n)
}

// Every way TOML nests must be rejected past the cap: inline tables, arrays,
// dotted keys, and table headers (CWE-770, vuln-0014).
func TestCheckRejectsDeepNesting(t *testing.T) {
	cases := map[string]string{
		"inline tables":    deep("{a=", "}", 4000),
		"arrays":           deep("[", "]", 4000),
		"dotted key":       strings.Repeat("a.", 3999) + "a = 1",
		"table header":     "[" + strings.Repeat("a.", 3999) + "a]\nv = 1",
		"array of tables":  "[[" + strings.Repeat("a.", 3999) + "a]]\nv = 1",
		"dotted in inline": "x = {" + strings.Repeat("a.", 3999) + "a = 1}",
	}
	for name, src := range cases {
		err := Check(src)
		var tooDeep *ErrTooDeep
		if !errors.As(err, &tooDeep) {
			t.Errorf("%s: Check = %v, want ErrTooDeep", name, err)
		}
	}
}

// A document at the cap passes; one step past it is rejected. Exercised for
// each nesting form so the boundary is covered everywhere.
func TestCheckBoundary(t *testing.T) {
	for _, form := range []struct {
		name       string
		at, beyond string
	}{
		{"inline", "x = " + strings.Repeat("{a=", MaxDepth) + "1" + strings.Repeat("}", MaxDepth),
			"x = " + strings.Repeat("{a=", MaxDepth+5) + "1" + strings.Repeat("}", MaxDepth+5)},
		{"array", "x = " + strings.Repeat("[", MaxDepth) + "1" + strings.Repeat("]", MaxDepth),
			"x = " + strings.Repeat("[", MaxDepth+5) + "1" + strings.Repeat("]", MaxDepth+5)},
	} {
		if err := Check(form.at); err != nil {
			t.Errorf("%s at the cap should pass, got: %v", form.name, err)
		}
		if err := Check(form.beyond); err == nil {
			t.Errorf("%s past the cap should be rejected", form.name)
		}
	}
}

// Realistic documents, and everything the decoder itself accepts at shallow
// depth, must pass unchanged — and still decode.
func TestCheckAllowsRealDocuments(t *testing.T) {
	docs := []string{
		`[database]
host = "localhost"
port = 5432
enabled = true
[[items]]
id = 1
[[items]]
id = 2
[deep.a.b.c]
arr = [1, 2, 3]
inline = {x = 1, y = {z = 2}}
`,
		`title = "config"
nested = {a = {b = {c = {d = 1}}}}
matrix = [[1, 2], [3, 4], [5, 6]]
`,
		"", // empty
		"# just a comment\n",
	}
	for _, d := range docs {
		if err := Check(d); err != nil {
			t.Errorf("Check rejected a valid document: %v\n%s", err, d)
		}
		var v any
		if _, err := toml.Decode(d, &v); err != nil {
			t.Errorf("the decoder rejected a document Check allowed: %v\n%s", err, d)
		}
	}
}

// Brackets and dots inside strings and comments must not be counted.
func TestCheckIgnoresStringsAndComments(t *testing.T) {
	for name, src := range map[string]string{
		"basic string":        `x = "` + strings.Repeat("{[.", 500) + `"`,
		"literal string":      `x = '` + strings.Repeat("{[.", 500) + `'`,
		"multiline basic":     "x = \"\"\"\n" + strings.Repeat("{[.\n", 500) + "\"\"\"",
		"multiline literal":   "x = '''\n" + strings.Repeat("{[.\n", 500) + "'''",
		"comment":             "# " + strings.Repeat("{[.", 500) + "\nx = 1",
		"escaped quote":       `x = "a\"` + strings.Repeat("{", 500) + `"`,
		"brackets then value": `name = "a.b.c.d.e.f"` + "\n" + `x = {y = 1}`,
	} {
		if err := Check(src); err != nil {
			t.Errorf("%s: brackets/dots in a string or comment were counted: %v", name, err)
		}
	}
}

// The guard runs before the decoder, so a deep document never reaches it: the
// check must be fast and allocate little regardless of depth.
func TestCheckIsCheap(t *testing.T) {
	src := deep("{a=", "}", 100000)
	for i := 0; i < 50; i++ {
		if err := Check(src); err == nil {
			t.Fatal("expected rejection")
		}
	}
}

// deepInline is an inline-table payload far past the cap, for placing after a
// string whose end the scanner might misjudge.
var deepInline = strings.Repeat("{a=", 2000) + "1" + strings.Repeat("}", 2000)

// A multi-line string may end with one or two extra quotes of its own kind
// (`"""x""""` is `x"`). The scanner must treat the whole run as the close; if it
// stops at the first three, the stray quote opens a fake single-line string
// that hides the rest of the line, and a deep payload after it slips through.
func TestCheckMultilineCloseWithExtraQuotes(t *testing.T) {
	for name, str := range map[string]string{
		`basic + 1 quote`:   `"""x""""`,
		`basic + 2 quotes`:  `"""x"""""`,
		`literal + 1 quote`: `'''x''''`,
		`literal + 2 quote`: `'''x'''''`,
	} {
		src := "a = [" + str + ", " + deepInline + "]"
		var tooDeep *ErrTooDeep
		if err := Check(src); !errors.As(err, &tooDeep) {
			t.Errorf("%s: Check = %v, want ErrTooDeep", name, err)
		}
	}
}

// The same strings in a legitimate document still pass and decode to the
// content the TOML spec gives them.
func TestCheckMultilineCloseWithExtraQuotesValid(t *testing.T) {
	for str, want := range map[string]string{
		`"""x""""`:  `x"`,
		`"""x"""""`: `x""`,
		`'''x''''`:  `x'`,
		`'''x'''''`: `x''`,
	} {
		src := "a = " + str + "\nb = {c = [1, 2]}\n"
		if err := Check(src); err != nil {
			t.Errorf("%s: Check rejected a valid document: %v", str, err)
			continue
		}
		var v struct {
			A string
			B struct{ C []int }
		}
		if _, err := toml.Decode(src, &v); err != nil {
			t.Errorf("%s: decode: %v", str, err)
			continue
		}
		if v.A != want || len(v.B.C) != 2 {
			t.Errorf("%s: decoded a=%q b=%v, want a=%q and b.c of 2", str, v.A, v.B, want)
		}
	}
}

// Other places a string scanner can lose track of where a string ends. Each
// is followed by a deep payload that must still be seen.
func TestCheckStringEdgesStillSeePayload(t *testing.T) {
	for name, src := range map[string]string{
		// Line-ending backslash inside a multi-line basic string is a
		// continuation, not an escape of the closing quotes.
		"line-ending backslash": "a = [\"\"\"x \\\n  y\"\"\", " + deepInline + "]",
		// A backslash right before the close escapes only the first quote.
		"escaped quote before close": `a = ["""x\""""", ` + deepInline + "]",
		// `#` inside a multi-line string is content, not a comment.
		"hash in multiline basic":   "a = [\"\"\"x # y\"\"\", " + deepInline + "]",
		"hash in multiline literal": "a = ['''x # y''', " + deepInline + "]",
		// The other quote kind inside a single-line string does not end it.
		"single quote in basic":   `a = ["it's", ` + deepInline + "]",
		"double quote in literal": `a = ['say "hi"', ` + deepInline + "]",
		"triple other quote":      `a = ["'''", '"""', ` + deepInline + "]",
		// An empty string is not the start of a multi-line one.
		"empty basic":   `a = ["", ` + deepInline + "]",
		"empty literal": `a = ['', ` + deepInline + "]",
		// An unterminated single-line string ends at the newline (a trailing
		// backslash does not continue it), and the next line is scanned fresh.
		"unterminated then deep key":  "a = \"x\n" + strings.Repeat("k.", 3000) + "k = 1",
		"backslash-newline then deep": "a = \"x\\\n" + "b = " + deepInline,
	} {
		var tooDeep *ErrTooDeep
		if err := Check(src); !errors.As(err, &tooDeep) {
			t.Errorf("%s: Check = %v, want ErrTooDeep", name, err)
		}
	}
}

// The same string forms in shallow documents pass and decode.
func TestCheckStringEdgesValid(t *testing.T) {
	for _, src := range []string{
		"a = \"\"\"x \\\n  y\"\"\"\nb = {c = 1}\n",
		`a = """x\""""` + "\nb = {c = 1}\n",
		"a = \"\"\"x # y\"\"\"\nb = {c = 1}\n",
		"a = '''x # y'''\nb = {c = 1}\n",
		`a = ["it's", 'say "hi"', "'''", '"""', "", '']` + "\nb = {c = 1}\n",
	} {
		if err := Check(src); err != nil {
			t.Errorf("Check rejected a valid document: %v\n%s", err, src)
		}
		var v any
		if _, err := toml.Decode(src, &v); err != nil {
			t.Errorf("decoder rejected a document Check allowed: %v\n%s", err, src)
		}
	}
}
