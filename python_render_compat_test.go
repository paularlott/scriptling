package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// Python render semantics across every conversion surface: f-strings,
// %-formatting, str.format, format(), str(), repr() and print() must all
// agree with Python 3 on how containers and nested instances render.
func TestContainerRenderMatchesPythonEverywhere(t *testing.T) {
	cases := map[string]struct{ script, want string }{
		"fstring plain list":        {"x = ['a', 'b']\nresult = f\"{x}\"", "['a', 'b']"},
		"fstring nested dict":       {"x = {'k': ['a']}\nresult = f\"{x}\"", "{'k': ['a']}"},
		"fstring !s list":           {"x = ['a']\nresult = f\"{x!s}\"", "['a']"},
		"fstring !r list":           {"x = ['a']\nresult = f\"{x!r}\"", "['a']"},
		"fstring alignment spec":    {"x = ['ab']\ntry:\n    result = f\"{x:>6}\"\nexcept TypeError as e:\n    result = str(e)", "unsupported format string passed to list.__format__"},
		"percent s list":            {"x = ['a', 'b']\nresult = \"%s\" % x", "['a', 'b']"},
		"percent r list":            {"x = ['a', 'b']\nresult = \"%r\" % x", "['a', 'b']"},
		"str.format list":           {"x = ['a']\nresult = \"{0}\".format(x)", "['a']"},
		"str.format exception":      {"try:\n    1/0\nexcept Exception as e:\n    result = '{0}'.format(e)", "division by zero"},
		"format builtin list":       {"try:\n    result = format(['ab'], '>6')\nexcept TypeError as e:\n    result = str(e)", "unsupported format string passed to list.__format__"},
		"str empty set":             {"result = str(set())", "set()"},
		"set nested in list":        {"s = set(['b', 'a'])\nresult = str([s, 'x'])", "[{'a', 'b'}, 'x']"},
		"str dict keys view":        {"result = str({'a': 1}.keys())", "dict_keys(['a'])"},
		"str dict values view":      {"result = str({'a': 1}.values())", "dict_values([1])"},
		"str dict items view":       {"result = str({'a': 1}.items())", "dict_items([('a', 1)])"},
		"str cyclic list":           {"l = []\nl.append(l)\nresult = str(l)", "[<cyclic reference>]"},
		"container uses repr":       {"class P:\n    def __str__(self):\n        return 'S'\n    def __repr__(self):\n        return 'R'\nresult = str([P()])", "[R]"},
		"fstring instance repr":     {"class P:\n    def __repr__(self):\n        return 'P!'\nresult = f\"{[P()]}\"", "[P!]"},
		"default instance in list":  {"class P:\n    pass\nresult = str([P()])", ""},
		"str exception message":     {"try:\n    1/0\nexcept Exception as e:\n    result = str(e)", "division by zero"},
		"fstring exception message": {"try:\n    1/0\nexcept Exception as e:\n    result = f\"{e}\"", "division by zero"},
		"str bool":                  {"result = str(True)", "True"},
		"fstring bytes":             {"result = f\"{b'abc'}\"", "b'abc'"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			if _, err := p.Eval(c.script); err != nil {
				t.Fatalf("Eval: %v\n%s", err, c.script)
			}
			got, gerr := p.GetVarAsString("result")
			if gerr != nil {
				t.Fatalf("result: %v", gerr)
			}
			if name == "default instance in list" {
				// Address is dynamic; check the shape only.
				if !strings.HasPrefix(got, "[<P object at 0x") || !strings.HasSuffix(got, ">]") {
					t.Fatalf("got %q, want [<P object at 0x...>]", got)
				}
				return
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// __getattr__ supplies attributes that normal lookup misses, both for
// attribute reads and calls, and may raise to report a miss.
func TestGetattrDunderFallback(t *testing.T) {
	cases := map[string]struct{ script, want string }{
		"read":      {"class P:\n    def __getattr__(self, name):\n        return 'dyn:' + name\nresult = P().anything", "dyn:anything"},
		"call":      {"class P:\n    def __getattr__(self, name):\n        return lambda: 'called:' + name\nresult = P().go()", "called:go"},
		"real wins": {"class P:\n    def __init__(self):\n        self.real = 'r'\n    def __getattr__(self, name):\n        return 'dyn'\nresult = P().real", "r"},
		"can raise": {"class P:\n    def __getattr__(self, name):\n        raise AttributeError('no ' + name)\nresult = 'no'\ntry:\n    P().x\n    result = 'no raise'\nexcept AttributeError as e:\n    result = str(e)", "no x"},
		"class dunderscore": {
			"class P:\n    pass\np = P()\nresult = p.__class__.__name__ + str(p.__class__ == P)",
			"PTrue",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := New()
			if _, err := p.Eval(c.script); err != nil {
				t.Fatalf("Eval: %v\n%s", err, c.script)
			}
			got, gerr := p.GetVarAsString("result")
			if gerr != nil {
				t.Fatalf("result: %v", gerr)
			}
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Missing attributes on built-in types raise catchable, Python-worded
// errors; subscripts on non-subscriptables raise TypeError.
func TestBuiltinTypeErrorsWordedLikePython(t *testing.T) {
	cases := map[string]struct{ expr, want string }{
		"list method":     {"[].foo()", "AttributeError: 'list' object has no attribute 'foo'"},
		"str attribute":   {"''.foo", "AttributeError: 'str' object has no attribute 'foo'"},
		"int method":      {"(1).foo()", "AttributeError: 'int' object has no attribute 'foo'"},
		"lambda attr":     {"(lambda: 1).foo", "AttributeError: 'function' object has no attribute 'foo'"},
		"builtin attr":    {"len.foo", "AttributeError: 'builtin_function_or_method' object has no attribute 'foo'"},
		"call int":        {"(5)()", "TypeError: 'int' object is not callable"},
		"subscript int":   {"(5)[0]", "TypeError: 'int' object is not subscriptable"},
		"super attribute": {"class A:\n    pass\nclass B(A):\n    def f(self):\n        return super().nope\nb = B()\nb.f()", "AttributeError: 'super' object has no attribute 'nope'"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			lines := strings.Split(c.expr, "\n")
			body := lines[len(lines)-1]
			prefix := strings.Join(lines[:len(lines)-1], "\n")
			script := prefix + "\nresult = 'no'\ntry:\n    " + body + "\n    result = 'no error'\nexcept AttributeError as e:\n    result = 'AttributeError: ' + str(e)\nexcept TypeError as e:\n    result = 'TypeError: ' + str(e)\n"
			p := New()
			stdlib.RegisterAll(p)
			if _, err := p.Eval(script); err != nil {
				t.Fatalf("Eval: %v\n%s", err, script)
			}
			got, _ := p.GetVarAsString("result")
			if got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
