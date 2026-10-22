package parser

import (
	"strings"
	"testing"
	"time"

	"github.com/paularlott/scriptling/ast"
	"github.com/paularlott/scriptling/lexer"
)

func parseSource(src string) (*ast.Program, []string) {
	p := New(lexer.New(src))
	prog := p.ParseProgram()
	return prog, p.Errors()
}

func ifBlocks(n int, body string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(strings.Repeat(" ", i) + "if 1:\n")
	}
	b.WriteString(strings.Repeat(" ", n) + body + "\n")
	return b.String()
}

// The parser had no nesting limit, and the passes that walk the tree after it
// recurse once per level, so a deep enough program overflowed the Go stack: a
// fatal error that ended the whole process (CWE-674, strix 4250 vuln-0009).
// Every deep shape is now one ordinary parse error, produced quickly.
func TestNestingLimitsRejectDeepPrograms(t *testing.T) {
	for name, tc := range map[string]struct{ src, want string }{
		"brackets":       {"x = " + strings.Repeat("[", 800000) + strings.Repeat("]", 800000), "too many nested parentheses"},
		"parens":         {"x = " + strings.Repeat("(", 800000) + "1" + strings.Repeat(")", 800000), "too many nested parentheses"},
		"dicts":          {"x = " + strings.Repeat("{1:", 800000) + "1" + strings.Repeat("}", 800000), "too many nested parentheses"},
		"attribute":      {"x = a" + strings.Repeat(".b", 800000), "expression too complex"},
		"subscript":      {"x = a" + strings.Repeat("[0]", 800000), "expression too complex"},
		"call":           {"x = a" + strings.Repeat("()", 800000), "expression too complex"},
		"binary":         {"x = a" + strings.Repeat("+a", 800000), "expression too complex"},
		"boolean":        {"x = a" + strings.Repeat(" or a", 800000), "expression too complex"},
		"unary":          {"x = " + strings.Repeat("-", 1600000) + "1", "expression too complex"},
		"one-line ifs":   {strings.Repeat("if 1: ", 800000) + "pass", "too many levels of indentation"},
		"indented ifs":   {ifBlocks(5000, "pass"), "too many levels of indentation"},
		"parens 201":     {"x = " + strings.Repeat("(", 201) + "1" + strings.Repeat(")", 201), "too many nested parentheses"},
		"indent 100":     {ifBlocks(100, "pass"), "too many levels of indentation"},
		"chain past cap": {"x = " + strings.Repeat("1+", maxExprDepth+10) + "1", "expression too complex"},
	} {
		start := time.Now()
		prog, errs := parseSource(tc.src)
		if len(errs) != 1 || !strings.Contains(errs[0], tc.want) {
			t.Errorf("%s: errors = %.3q, want one %q", name, errs, tc.want)
		}
		if len(prog.Statements) != 0 {
			t.Errorf("%s: a rejected program kept %d statements", name, len(prog.Statements))
		}
		if d := time.Since(start); d > 5*time.Second {
			t.Errorf("%s: took %v", name, d)
		}
	}
}

// The limits match CPython 3.14, so anything CPython compiles still parses:
// 200 nested brackets, 99 nested blocks, 10,000-term chains, 1,000 unary
// operators.
func TestNestingLimitsAllowWhatPythonAllows(t *testing.T) {
	for name, src := range map[string]string{
		"parens 200":         "x = " + strings.Repeat("(", 200) + "1" + strings.Repeat(")", 200),
		"lists 200":          "x = " + strings.Repeat("[", 200) + strings.Repeat("]", 200),
		"indent 99":          ifBlocks(99, "pass"),
		"binary chain 10000": "x = " + strings.Repeat("1+", 9999) + "1",
		"attr chain 10000":   "x = a" + strings.Repeat(".b", 10000),
		"call chain 10000":   "x = a" + strings.Repeat("()", 10000),
		"unary 1000":         "x = " + strings.Repeat("-", 1000) + "1",
		"many statements":    strings.Repeat("x = (((1)))\n", 50000),
		"many siblings":      "x = [" + strings.Repeat("[[1]], ", 50000) + "]",
	} {
		if _, errs := parseSource(src); len(errs) != 0 {
			t.Errorf("%s: unexpected errors %.3q", name, errs)
		}
	}
}
