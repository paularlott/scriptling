package parser

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/lexer"
)

func parseErrors(src string) []string {
	p := New(lexer.New(src))
	p.ParseProgram()
	return p.Errors()
}

// Source Python refuses is refused: statements run together on a line, and
// lines indented where no block starts.
func TestLayoutRefused(t *testing.T) {
	for name, tc := range map[string]struct{ src, want string }{
		"two assignments":            {"a = 1b = 2\n", `line 1: invalid syntax: "b" follows a statement`},
		"two calls":                  {"print(1)print(2)\n", `line 1: invalid syntax: "print" follows a statement`},
		"names with a space":         {"x = 1 y = 2\n", `line 1: invalid syntax: "y" follows a statement`},
		"after a string":             {"s = 'a' t = 2\n", `line 1: invalid syntax: "t" follows`},
		"after a subscript":          {"x = [1][0] y = 2\n", `line 1: invalid syntax: "y" follows`},
		"after an attribute":         {"import os\nx = os.sep y = 2\n", `line 2: invalid syntax: "y" follows`},
		"after return":               {"def f():\n    return 1 2\n", `line 2: invalid syntax: "2" follows`},
		"in a one-line block":        {"if True: a = 1 b = 2\n", `line 1: invalid syntax: "b" follows`},
		"in a block":                 {"if True:\n    a = 1\n    b = 2 c = 3\n", `line 3: invalid syntax: "c" follows`},
		"a digit-led name":           {"123name = 1\n", `line 1: invalid syntax: "name" follows`},
		"indented line":              {"x = 1\n    y = 2\n", "line 2: unexpected indent"},
		"indented in a block":        {"if True:\n    a = 1\n        b = 2\n", "line 3: unexpected indent"},
		"indented after a one-liner": {"if True: a = 1\n    b = 2\n", "line 2: unexpected indent"},
		"indented after a function":  {"def f():\n    return 1\n\n        x = 2\n", "line 4: unexpected indent"},
		"script starts indented":     {"    x = 1\n    y = 2\n", "line 1: unexpected indent: the script starts indented"},
		"script starts indented later": {
			"\n\n\tx = 1\n", "line 3: unexpected indent: the script starts indented"},
	} {
		t.Run(name, func(t *testing.T) {
			errs := parseErrors(tc.src)
			if len(errs) == 0 || !strings.HasPrefix(errs[0], tc.want) {
				t.Errorf("errors %q, want one starting %q", errs, tc.want)
			}
		})
	}
}

// What Python takes is still taken: separators, continuations, brackets,
// blank and comment lines at any indentation, and the usual compound forms.
func TestLayoutAccepted(t *testing.T) {
	for name, src := range map[string]string{
		"semicolons":                 "a = 1; b = 2; print(a, b)\n",
		"trailing semicolon":         "a = 1;\n",
		"one-line block, semicolons": "if True: a = 1; b = 2\n",
		"one-line blocks, else":      "if False: a = 1\nelif True: a = 2\nelse: a = 3\n",
		"backslash continuation":     "x = 1 + \\\n    2\n",
		"bracket continuation":       "x = (\n  1,\n      2,\n)\ny = [\n    1,\n]\nz = {\n 'a': 1,\n        'b': 2}\n",
		"call over lines":            "print(\n    1,\n        2)\n",
		"implicit string join":       "s = ('a'\n     'b')\n",
		"triple-quoted":              "s = '''\n    indented\n        text\n'''\nt = 1\n",
		"blank lines with spaces":    "x = 1\n    \n\t\ny = 2\n",
		"comments at any indent":     "x = 1\n        # deep comment\n# shallow\ny = 2\n",
		"comment after a block":      "if True:\n    a = 1\n            # comment\nb = 2\n",
		"trailing whitespace at EOF": "x = 1\n    ",
		"no newline at EOF":          "x = 1",
		"CRLF with blank lines":      "x = 1\r\n    \r\nif x:\r\n    y = 2\r\n    \r\nz = 3\r\n",
		"nested dedents":             "def f():\n    if True:\n        for i in []:\n            pass\n    return 1\nx = f()\n",
		"class and decorators":       "def d(f):\n    return f\n\nclass C:\n    @d\n    def m(self):\n        return 1\n\nc = C()\n",
		"try":                        "try:\n    x = 1\nexcept Exception as e:\n    x = 2\nelse:\n    x = 3\nfinally:\n    y = 4\n",
		"loops with else":            "for i in []:\n    pass\nelse:\n    x = 1\nwhile False:\n    pass\nelse:\n    y = 2\n",
		"with":                       "class M:\n    def __enter__(self):\n        return self\n    def __exit__(self, *a):\n        return False\nwith M() as m:\n    x = 1\n",
		"match":                      "match 1:\n    case 1:\n        x = 1\n    case _:\n        x = 2\n",
		"lambda and comprehension":   "f = lambda x: x + 1\nxs = [f(i) for i in range(3) if i]\n",
		"lazy import":                "lazy import json\nlazy\nimport json\n",
		"block then statement":       "if True:\n    a = 1\nb = 2\n",
		"one-line def":               "def f(): return 1\nx = f()\n",
		"walrus":                     "if (n := 3) > 2:\n    x = n\n",
	} {
		t.Run(name, func(t *testing.T) {
			if errs := parseErrors(src); len(errs) != 0 {
				t.Errorf("errors %q", errs)
			}
		})
	}
}

// An expression may start with spaces: f-string fields are parsed on their own.
func TestLayoutExpressionsMayStartIndented(t *testing.T) {
	for _, src := range []string{
		"x = 2\ns = f'{ x }'\n",
		"x = 2\ns = f'{  x + 1 = }'\n",
		"x = 2\ns = f'{ x :>4}'\n",
	} {
		if errs := parseErrors(src); len(errs) != 0 {
			t.Errorf("%q: %q", src, errs)
		}
	}
	if e := ParseExpressionString("  1 + 2"); e == nil {
		t.Error("an expression starting with spaces did not parse")
	}
}
