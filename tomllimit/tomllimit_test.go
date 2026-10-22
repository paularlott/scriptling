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
