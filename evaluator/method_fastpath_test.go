package evaluator

import (
	"testing"

	"github.com/paularlott/scriptling/object"
)

// tryEvalInstanceMethodFast short-circuits `obj.method(...)` for plain class
// methods called with positional arguments. Everything it does not cover has to
// fall through to callInstanceMethod unchanged, and getting that wrong would
// silently bind the wrong receiver or the wrong parameter — so each excluded
// shape is covered here.

func TestMethodFastPathBasicDispatch(t *testing.T) {
	cases := []struct{ name, src, want string }{
		{"no args", `
class C:
    def ping(self):
        return "pong"
result = C().ping()
`, "pong"},
		{"one arg", `
class C:
    def echo(self, v):
        return v
result = C().echo("x")
`, "x"},
		{"two args", `
class C:
    def add(self, a, b):
        return a + b
result = str(C().add(2, 3))
`, "5"},
		{"three args", `
class C:
    def join3(self, a, b, c):
        return a + b + c
result = C().join3("a", "b", "c")
`, "abc"},
		{"four args falls back", `
class C:
    def join4(self, a, b, c, d):
        return a + b + c + d
result = C().join4("a", "b", "c", "d")
`, "abcd"},
		{"self is the right instance", `
class C:
    def __init__(self, tag):
        self.tag = tag
    def get(self):
        return self.tag
a = C("a")
b = C("b")
result = a.get() + b.get() + a.get()
`, "aba"},
		{"mutating self through a method", `
class Counter:
    def __init__(self):
        self.n = 0
    def bump(self, by):
        self.n = self.n + by
        return self.n
c = Counter()
c.bump(2)
c.bump(3)
result = str(c.bump(5))
`, "10"},
		{"nested method calls", `
class C:
    def double(self, v):
        return v * 2
    def quad(self, v):
        return self.double(self.double(v))
result = str(C().quad(3))
`, "12"},
		{"recursive method", `
class C:
    def fact(self, n):
        if n <= 1:
            return 1
        return n * self.fact(n - 1)
result = str(C().fact(5))
`, "120"},
		{"inherited method", `
class Base:
    def name(self):
        return "base"
class Child(Base):
    pass
result = Child().name()
`, "base"},
		{"overridden method", `
class Base:
    def name(self):
        return "base"
class Child(Base):
    def name(self):
        return "child"
result = Child().name()
`, "child"},
		{"method calling super", `
class Base:
    def name(self):
        return "base"
class Child(Base):
    def name(self):
        return "child+" + super().name()
result = Child().name()
`, "child+base"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := evalSrc(t, c.src); got.Inspect() != c.want {
				t.Errorf("got %q want %q", got.Inspect(), c.want)
			}
		})
	}
}

func TestMethodFastPathDeclinesSpecialForms(t *testing.T) {
	// Each of these must reach the general path and keep its existing behaviour.
	cases := []struct{ name, src, want string }{
		{"instance field shadows method and takes no self", `
class C:
    def f(self):
        return "method"
c = C()
c.f = lambda: "field"
result = c.f()
`, "field"},
		{"staticmethod gets no self", `
class C:
    @staticmethod
    def s(a, b):
        return a + b
result = str(C().s(1, 2))
`, "3"},
		{"classmethod gets the class", `
class C:
    @classmethod
    def make(cls, v):
        return "cls:" + v
result = C().make("x")
`, "cls:x"},
		{"default argument", `
class C:
    def greet(self, who="world"):
        return "hi " + who
result = C().greet() + "|" + C().greet("you")
`, "hi world|hi you"},
		{"keyword argument", `
class C:
    def wrap(self, text, prefix="["):
        return prefix + text
result = C().wrap("x", prefix=">")
`, ">x"},
		{"variadic", `
class C:
    def total(self, *nums):
        t = 0
        for n in nums:
            t = t + n
        return t
result = str(C().total(1, 2, 3, 4))
`, "10"},
		{"kwargs", `
class C:
    def collect(self, **kw):
        return str(kw["a"]) + "," + str(kw["b"])
result = C().collect(b=1, a=2)
`, "2,1"},
		{"args unpacking", `
class C:
    def add(self, a, b):
        return a + b
vals = [1, 2]
result = str(C().add(*vals))
`, "3"},
		{"kwargs unpacking", `
class C:
    def add(self, a, b):
        return a + b
d = {"a": 1, "b": 2}
result = str(C().add(**d))
`, "3"},
		{"property is not callable via the fast path", `
class C:
    @property
    def value(self):
        return "v"
result = C().value
`, "v"},
		{"method stored as a field on the instance", `
class C:
    def f(self, v):
        return "f" + v
c = C()
g = c.f
result = g("x")
`, "fx"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := evalSrc(t, c.src); got.Inspect() != c.want {
				t.Errorf("got %q want %q", got.Inspect(), c.want)
			}
		})
	}
}

func TestMethodFastPathErrorHandling(t *testing.T) {
	// Errors in the receiver, in an argument, and in the body must all surface.
	for _, src := range []string{
		"class C:\n    def f(self, v):\n        return v\nresult = C().f(missing)\n",
		"class C:\n    def f(self, a, b):\n        return a\nresult = C().f(1, missing)\n",
		"class C:\n    def f(self, a, b, c):\n        return a\nresult = C().f(1, 2, missing)\n",
		"class C:\n    def f(self):\n        return undefined_name\nresult = C().f()\n",
		"class C:\n    def f(self, v):\n        return v\nresult = C().nosuch(1)\n",
		// Wrong arity must still be reported, not silently accepted.
		"class C:\n    def f(self, a, b):\n        return a\nresult = C().f(1)\n",
		"class C:\n    def f(self, a):\n        return a\nresult = C().f(1, 2)\n",
	} {
		got := evalSrc(t, src)
		if !isErrorLike(got) {
			t.Errorf("%q: expected an error, got %s (%s)", src, got.Type(), got.Inspect())
		}
	}
}

func TestMethodFastPathExceptionsPropagate(t *testing.T) {
	// A raise inside a fast-path method must still be catchable by the caller.
	src := `
class C:
    def boom(self, v):
        raise ValueError("bad " + v)
result = "not run"
try:
    C().boom("input")
except ValueError as e:
    result = str(e)
`
	if got := evalSrc(t, src); got.Inspect() != "bad input" {
		t.Errorf("got %q want %q", got.Inspect(), "bad input")
	}
}

func TestMethodFastPathReturnsAndControlFlow(t *testing.T) {
	// Early return, loops and break inside a fast-path method body.
	src := `
class C:
    def firstEven(self, limit):
        for i in range(limit):
            if i > 0 and i % 2 == 0:
                return i
        return -1
result = str(C().firstEven(10)) + "," + str(C().firstEven(2))
`
	if got := evalSrc(t, src); got.Inspect() != "2,-1" {
		t.Errorf("got %q want %q", got.Inspect(), "2,-1")
	}
}

// isErrorLike reports whether obj is an error or a raised exception.
func isErrorLike(obj object.Object) bool {
	return object.IsError(obj) || obj.Type() == object.EXCEPTION_OBJ
}
