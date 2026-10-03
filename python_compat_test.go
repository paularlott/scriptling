package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// pyEvalString runs script and returns the string variable result.
func pyEvalString(t *testing.T, script string) string {
	t.Helper()
	p := New()
	if _, err := p.Eval(script); err != nil {
		t.Fatalf("Eval: %v\n%s", err, script)
	}
	v, objErr := p.GetVarAsString("result")
	if objErr != nil {
		t.Fatalf("result: %v", objErr)
	}
	return v
}

func TestMissingAttributeRaisesAttributeError(t *testing.T) {
	cases := map[string]struct{ script, want string }{
		"instance": {
			"class P:\n    pass\ntry:\n    P().nope\n    result = \"no raise\"\nexcept AttributeError as e:\n    result = str(e)\n",
			"'P' object has no attribute 'nope'",
		},
		"class": {
			"class P:\n    pass\ntry:\n    P.nope\n    result = \"no raise\"\nexcept AttributeError as e:\n    result = str(e)\n",
			"type object 'P' has no attribute 'nope'",
		},
		"method call": {
			"class P:\n    pass\ntry:\n    P().missing()\n    result = \"no raise\"\nexcept AttributeError as e:\n    result = str(e)\n",
			"'P' object has no attribute 'missing'",
		},
		"after del": {
			"class P:\n    def __init__(self):\n        self.x = 1\np = P()\ndel p.x\ntry:\n    p.x\n    result = \"no raise\"\nexcept AttributeError as e:\n    result = str(e)\n",
			"'P' object has no attribute 'x'",
		},
		"hasattr unaffected": {
			"class P:\n    y = 1\nresult = str(hasattr(P(), \"nope\")) + str(hasattr(P(), \"y\"))\n",
			"FalseTrue",
		},
		"class attribute via instance": {
			"class P:\n    y = 'cls'\nresult = P().y\n",
			"cls",
		},
		"__class__": {
			"class P:\n    pass\np = P()\nresult = p.__class__.__name__ + str(p.__class__ == P)\n",
			"PTrue",
		},
		"__getattr__ fallback": {
			"class P:\n    def __init__(self):\n        self.real = 'r'\n    def __getattr__(self, name):\n        return 'dyn:' + name\np = P()\nresult = p.real + ' ' + p.other\n",
			"r dyn:other",
		},
		"__getattr__ can raise": {
			"class P:\n    def __getattr__(self, name):\n        raise AttributeError('no ' + name)\ntry:\n    P().x\n    result = 'no raise'\nexcept AttributeError as e:\n    result = str(e)\n",
			"no x",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := pyEvalString(t, c.script); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestBuiltinTypeAttributeAndCallErrors(t *testing.T) {
	cases := map[string]struct{ expr, want string }{
		"list method":     {"[].foo()", "AttributeError: 'list' object has no attribute 'foo'"},
		"dict method":     {"{}.foo()", "AttributeError: 'dict' object has no attribute 'foo'"},
		"str method":      {"''.foo()", "AttributeError: 'str' object has no attribute 'foo'"},
		"int method":      {"(1).foo()", "AttributeError: 'int' object has no attribute 'foo'"},
		"tuple method":    {"(1,).foo()", "AttributeError: 'tuple' object has no attribute 'foo'"},
		"list attribute":  {"[].foo", "AttributeError: 'list' object has no attribute 'foo'"},
		"lambda attr":     {"(lambda: 1).foo", "AttributeError: 'function' object has no attribute 'foo'"},
		"builtin attr":    {"len.foo", "AttributeError: 'builtin_function_or_method' object has no attribute 'foo'"},
		"call int":        {"(5)()", "TypeError: 'int' object is not callable"},
		"subscript int":   {"(5)[0]", "TypeError: 'int' object is not subscriptable"},
		"module function": {"import json\njson.nope()", "AttributeError: module 'json' has no attribute 'nope'"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			lines := strings.Split(c.expr, "\n")
			body := lines[len(lines)-1]
			prefix := strings.Join(lines[:len(lines)-1], "\n")
			script := prefix + "\ntry:\n    " + body + "\n    result = 'no error'\nexcept AttributeError as e:\n    result = 'AttributeError: ' + str(e)\nexcept TypeError as e:\n    result = 'TypeError: ' + str(e)\n"
			p := New()
			stdlib.RegisterAll(p)
			if _, err := p.Eval(script); err != nil {
				t.Fatalf("Eval: %v\n%s", err, script)
			}
			if got, _ := p.GetVarAsString("result"); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Review follow-ups: getattr/hasattr use dot-access semantics, Python type
// names, module attributes, the format protocol, exception repr.
func TestPythonCompatReviewFixes(t *testing.T) {
	cases := map[string]struct{ script, want string }{
		"getattr __getattr__": {
			"class P:\n    def __getattr__(self, n):\n        if n.startswith('d'):\n            return 'dyn:' + n\n        raise AttributeError(n)\np = P()\nresult = getattr(p, 'dx') + str(hasattr(p, 'dx')) + str(hasattr(p, 'zz')) + getattr(p, 'zz', '-')\n",
			"dyn:dxTrueFalse-",
		},
		"getattr bound method and property": {
			"class P:\n    def __init__(self):\n        self.x = 1\n    def m(self):\n        return self.x + 1\n    @property\n    def a(self):\n        return 9\np = P()\nresult = str(getattr(p, 'm')()) + str(getattr(p, 'a')) + getattr(p, '__class__').__name__\n",
			"29P",
		},
		"hasattr propagates other errors": {
			"class R:\n    def __getattr__(self, n):\n        raise ValueError('boom')\ntry:\n    hasattr(R(), 'z')\n    result = 'swallowed'\nexcept ValueError as e:\n    result = str(e)\n",
			"boom",
		},
		"python type names": {
			"result = ''\nfor v in [b'x', {}.keys(), ValueError('v'), len]:\n    try:\n        v.nope\n    except AttributeError as e:\n        result += str(e).split(' ')[0] + ' '\n",
			"'bytes' 'dict_keys' 'ValueError' 'builtin_function_or_method' ",
		},
		"module attribute": {
			"import math\ntry:\n    math.nope\n    result = 'no error'\nexcept AttributeError:\n    result = 'AttributeError'\ntry:\n    {}['k']\nexcept KeyError:\n    result += ' KeyError'\n",
			"AttributeError KeyError",
		},
		"__format__": {
			"class F:\n    def __format__(self, spec):\n        return 'F<' + spec + '>'\nresult = f'{F():x}' + format(F(), 'y') + '{:z}'.format(F())\n",
			"F<x>F<y>F<z>",
		},
		"format spec rejected for unsupported types": {
			"result = ''\nfor v in [[1], None, {}]:\n    try:\n        f'{v:>6}'\n    except TypeError as e:\n        result += str(e) + ';'\n",
			"unsupported format string passed to list.__format__;unsupported format string passed to NoneType.__format__;unsupported format string passed to dict.__format__;",
		},
		"empty spec still str": {
			"class P:\n    def __str__(self):\n        return 'p'\nresult = f'{P()}|{[1]}|{None}|' + format([1])\n",
			"p|[1]|None|[1]",
		},
		"exception repr": {
			"e = ValueError('bad')\nresult = repr(e) + ' ' + f'{e!r}' + ' ' + str([e]) + ' ' + str(e) + ' ' + repr(KeyError())\n",
			"ValueError('bad') ValueError('bad') [ValueError('bad')] bad KeyError()",
		},
		"f-string expressions": {
			"items = [1, 2, 3, 4]\na, b = 1, 2\nd = {'a:b': 5}\nresult = f\"{items[1:3]}|{a != b}|{d['a:b']}|{ {'x': 1}['x'] }|{(lambda q: q * 2)(3)}|{'{}'.format(7)}|{items[1:3]!r:>8}\"\n",
			"[2, 3]|True|5|1|6|7|  [2, 3]",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			if _, err := p.Eval(c.script); err != nil {
				t.Fatalf("Eval: %v\n%s", err, c.script)
			}
			if got, _ := p.GetVarAsString("result"); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

// Second review: index errors, instance subscripts, modules, f-string brackets.
func TestPythonCompatSecondReview(t *testing.T) {
	cases := map[string]struct{ script, want string }{
		"sequence index type": {
			"result = ''\nfor f in [lambda: ['x']['a'], lambda: 'abc'['a'], lambda: (1,)['a'], lambda: (5)[0]]:\n    try:\n        f()\n    except TypeError as e:\n        result += str(e) + ';'\n",
			"list indices must be integers or slices, not str;string indices must be integers, not 'str';tuple indices must be integers or slices, not str;'int' object is not subscriptable;",
		},
		"instance subscript": {
			"class P:\n    def __init__(self):\n        self.k = 1\n    def __getattr__(self, n):\n        return 'dyn'\ntry:\n    P()['k']\n    result = 'no error'\nexcept TypeError as e:\n    result = str(e)\n",
			"'P' object is not subscriptable",
		},
		"inherited __getitem__": {
			"class B:\n    def __getitem__(self, k):\n        return 'got ' + str(k)\nclass S(B):\n    pass\nresult = S()['x']\n",
			"got x",
		},
		"modules": {
			"import math\nresult = str(math) + ' ' + str(hasattr(math, 'keys')) + ' ' + str(hasattr({}, 'keys'))\ntry:\n    math.keys()\nexcept AttributeError as e:\n    result += ' ' + str(e)\nresult += ' ' + str('keys' in dir({}))\n",
			"<module 'math'> False True module 'math' has no attribute 'keys' True",
		},
		"f-string stray bracket stays local": {
			"a, b = 1, 2\nresult = f'{a)} {b}'\n",
			"1 2",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			if _, err := p.Eval(c.script); err != nil {
				t.Fatalf("Eval: %v\n%s", err, c.script)
			}
			if got, _ := p.GetVarAsString("result"); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestReprControlCharsAndFloatArrayDisplay(t *testing.T) {
	cases := map[string]struct{ script, want string }{
		"repr control chars": {
			"import string\nresult = repr(string.whitespace) + ' ' + repr(chr(0) + chr(127) + chr(27))\n",
			`' \t\n\r\x0b\x0c' '\x00\x7f\x1b'`,
		},
		"float array floats": {
			"import math\nm = math.array([[1.0, 2.0], [3.0, 4.5]])\nresult = str(m[0]) + ' ' + str(m)\n",
			"[1.0, 2.0] [[1.0, 2.0], [3.0, 4.5]]",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			if _, err := p.Eval(c.script); err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if got, _ := p.GetVarAsString("result"); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestCollectionsAndPrintPythonBehaviour(t *testing.T) {
	cases := map[string]struct{ script, want string }{
		"bounded deque drops from the opposite end": {
			"import collections\nd = collections.deque([1, 2], maxlen=2)\nd.append(3)\na = str(d)\nd.appendleft(0)\nb = str(d)\nd.extend([7, 8, 9])\nresult = a + ' ' + b + ' ' + str(d)\n",
			"deque([2, 3], maxlen=2) deque([0, 2], maxlen=2) deque([8, 9], maxlen=2)",
		},
		"dict kwargs override mapping": {
			"result = str(dict({'a': 1}, a=2))\n",
			"{'a': 2}",
		},
		"counter repr": {
			"import collections\nresult = str(collections.Counter('aab')) + ' ' + repr(collections.Counter())\n",
			"Counter({'a': 2, 'b': 1}) Counter()",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			if _, err := p.Eval(c.script); err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if got, _ := p.GetVarAsString("result"); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}

	p := New()
	p.EnableOutputCapture()
	if _, err := p.Eval("class A:\n    def __repr__(self):\n        return 'A()'\nprint(A())\n"); err != nil {
		t.Fatal(err)
	}
	if got := p.GetOutput(); got != "A()\n" {
		t.Fatalf("print should fall back to __repr__, got %q", got)
	}
}

// collections.Counter is a dict of element -> count with Python's API.
func TestCounterIsAPythonCounter(t *testing.T) {
	cases := map[string]struct{ script, want string }{
		"real keys":     {"import collections\nc = collections.Counter([3, 3, 1])\nresult = str(c) + ' ' + str(c[3]) + ' ' + str(c[9]) + ' ' + str(c.most_common(1))\n", "Counter({3: 2, 1: 1}) 2 0 [(3, 2)]"},
		"dict api":      {"import collections\nc = collections.Counter('aab')\nresult = str([len(c), 'a' in c, sorted(c.items()), sorted(c), c.get('z', 0), c.total()])\n", "[2, True, [('a', 2), ('b', 1)], ['a', 'b'], 0, 3]"},
		"update":        {"import collections\nc = collections.Counter(a=1)\nc.update(['a', 'b'])\nc.subtract({'b': 3})\nc['z'] = 4\ndel c['z']\nresult = str(c)\n", "Counter({'a': 2, 'b': -2})"},
		"arithmetic":    {"import collections\na = collections.Counter(a=3, b=1)\nb = collections.Counter(a=1, b=2)\nresult = str([a + b, a - b, a | b, a & b])\n", "[Counter({'a': 4, 'b': 3}), Counter({'a': 2}), Counter({'a': 3, 'b': 2}), Counter({'a': 1, 'b': 1})]"},
		"equality":      {"import collections\nresult = str(collections.Counter('ab') == collections.Counter({'a': 1, 'b': 1}))\n", "True"},
		"module helper": {"import collections\nresult = str(collections.most_common(collections.Counter('aab'), 1))\n", "[('a', 2)]"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := New()
			stdlib.RegisterAll(p)
			if _, err := p.Eval(c.script); err != nil {
				t.Fatalf("Eval: %v", err)
			}
			if got, _ := p.GetVarAsString("result"); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}
