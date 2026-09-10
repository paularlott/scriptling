package scriptling

import (
	"strings"
	"testing"

	"github.com/paularlott/scriptling/stdlib"
)

// This file pins down the raised-vs-value exception semantics introduced with
// object.Exception.Raised: an exception produced by `raise` (or by a failing
// operation) has Raised=true and unwinds the stack from every sub-expression
// position, while a constructed or caught exception (Raised=false) is an
// ordinary value. Each test covers both the unhappy path (a raise must
// propagate) and the happy path (normal operation, or the same code shape with
// a mere value must NOT raise).

// next() signals exhaustion by raising StopIteration: it must be catchable,
// must surface uncaught, and must honour the two-argument default form.
func TestNextStopIterationRaiseAndDefault(t *testing.T) {
	t.Run("exhausted next is catchable as StopIteration", func(t *testing.T) {
		got := evalString(t, `
it = iter([1])
a = next(it)
r = "a=" + str(a)
try:
    next(it)
    r = r + "/not-raised"
except StopIteration:
    r = r + "/caught"
r
`)
		if got != "a=1/caught" {
			t.Fatalf("result = %q, want %q", got, "a=1/caught")
		}
	})

	t.Run("exhausted next surfaces uncaught", func(t *testing.T) {
		p := New()
		_, err := p.Eval(`next(iter([]))`)
		if err == nil {
			t.Fatal("expected uncaught StopIteration to return an Eval error")
		}
		if !strings.Contains(err.Error(), "StopIteration") {
			t.Fatalf("error = %q, want it to mention StopIteration", err.Error())
		}
	})

	t.Run("next with default returns default instead of raising", func(t *testing.T) {
		got := evalString(t, `next(iter([]), "fallback")`)
		if got != "fallback" {
			t.Fatalf("result = %q, want %q", got, "fallback")
		}
	})

	t.Run("next yields successive elements", func(t *testing.T) {
		got := evalString(t, `
it = iter([7, 8])
str(next(it)) + "," + str(next(it))
`)
		if got != "7,8" {
			t.Fatalf("result = %q, want %q", got, "7,8")
		}
	})
}

// Adding or constructing a set with an unhashable element raises TypeError
// (evalSetAdd marks the exception Raised so it can be caught).
func TestSetUnhashableRaisesTypeError(t *testing.T) {
	t.Run("unhashable set literal element", func(t *testing.T) {
		got := evalString(t, `
try:
    s = {1, [2]}
    r = "not-raised"
except TypeError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("set.add with unhashable argument", func(t *testing.T) {
		got := evalString(t, `
s = set([1])
try:
    s.add([1, 2])
    r = "not-raised"
except TypeError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("set.add with hashable argument still works", func(t *testing.T) {
		got := evalString(t, `
s = set([1])
s.add(2)
len(s)
`)
		if got != "2" {
			t.Fatalf("result = %q, want %q", got, "2")
		}
	})
}

// sorted()'s key function: a raise inside it must propagate; a working key
// must still sort (both directions through the changed guard in
// sortedFunctionImpl).
func TestSortedKeyRaisePropagation(t *testing.T) {
	t.Run("raising key propagates", func(t *testing.T) {
		got := evalString(t, `
def bad_key(x):
    raise ValueError("bad key")
try:
    sorted([3, 1, 2], key=bad_key)
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("working key still sorts", func(t *testing.T) {
		got := evalString(t, `sorted([3, 1, 2], key=lambda x: -x)`)
		if got != "[3, 2, 1]" {
			t.Fatalf("result = %q, want %q", got, "[3, 2, 1]")
		}
	})
}

// Tuple and set literals must propagate a raised element exactly like list
// literals do (isPropagatedError over evalExpressionsWithContext).
func TestRaisedInTupleAndSetLiteralsPropagates(t *testing.T) {
	cases := map[string]string{
		"tuple literal": "t = (1, boom(), 3)",
		"set literal":   "s = {1, boom(), 3}",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			got := evalString(t, `
def boom():
    raise ValueError("x")
caught = "no"
try:
    `+stmt+`
except ValueError:
    caught = "yes"
caught
`)
			if got != "yes" {
				t.Fatalf("%s: result = %q, want %q", name, got, "yes")
			}
		})
	}
}

// A raise in any argument position of a call — keyword argument, method
// keyword argument, *args unpacking, **kwargs unpacking — and in the receiver
// position of a method call must propagate to an enclosing except.
func TestRaisedInCallFormsPropagates(t *testing.T) {
	cases := map[string]string{
		"keyword arg":        `f(x=boom())`,
		"method keyword arg": `c.m(x=boom())`,
		"star-args unpack":   `varf(*boom())`,
		"kwargs unpack":      `kwf(**boom())`,
		"method receiver":    `boom().append(1)`,
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			got := evalString(t, `
def boom():
    raise ValueError("x")
def f(x):
    return x
def varf(*a):
    return a
def kwf(**k):
    return k
class C:
    def m(self, x):
        return x
c = C()
caught = "no"
try:
    `+stmt+`
except ValueError:
    caught = "yes"
caught
`)
			if got != "yes" {
				t.Fatalf("%s: result = %q, want %q", name, got, "yes")
			}
		})
	}

	t.Run("same forms with plain values do not raise", func(t *testing.T) {
		got := evalString(t, `
class C:
    def m(self, x):
        return x
def f(x=0, *a, **k):
    return x
c = C()
str(f(x=1)) + "," + str(c.m(x=2))
`)
		if got != "1,2" {
			t.Fatalf("result = %q, want %q", got, "1,2")
		}
	})
}

// A raise on the right-hand side of augmented and chained assignments, and in
// the index/target of a del statement, must propagate as that exception.
func TestRaisedInAssignmentAndDelPropagates(t *testing.T) {
	cases := map[string]string{
		"augmented assignment": `x += boom()`,
		"chained assignment":   `a = b = boom()`,
		"del index target":     `del d[boom()]`,
		"del slice bound":      `del l[boom():]`,
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			got := evalString(t, `
def boom():
    raise ValueError("bang")
x = 1
d = {"k": 1}
l = [1, 2, 3, 4, 5]
caught = "no"
try:
    `+stmt+`
except ValueError as e:
    caught = "caught:" + str(e)
caught
`)
			if got != "caught:bang" {
				t.Fatalf("%s: result = %q, want %q", name, got, "caught:bang")
			}
		})
	}

	t.Run("same assignment forms with values do not raise", func(t *testing.T) {
		got := evalString(t, `
e = ValueError("v")   # a value, not a raise
x = 1
x += 1
a = b = e
"ok:" + str(a)
`)
		if got != "ok:v" {
			t.Fatalf("result = %q, want %q", got, "ok:v")
		}
	})
}

// A raise in the operand of a prefix expression (-, not) must propagate.
func TestRaisedInPrefixOperandPropagates(t *testing.T) {
	for _, op := range []string{"-", "not "} {
		t.Run("operator "+strings.TrimSpace(op), func(t *testing.T) {
			got := evalString(t, `
def boom():
    raise ValueError("x")
caught = "no"
try:
    v = `+op+`boom()
except ValueError:
    caught = "yes"
caught
`)
			if got != "yes" {
				t.Fatalf("operator %q: result = %q, want %q", op, got, "yes")
			}
		})
	}
}

// dict.get() fast path: a raised key or a raised default must propagate;
// the happy path must keep returning the value/default.
func TestDictGetRaisedKeyAndDefaultPropagate(t *testing.T) {
	t.Run("raised key propagates", func(t *testing.T) {
		got := evalString(t, `
def boom():
    raise ValueError("x")
d = {"a": 1}
try:
    d.get(boom())
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("raised default propagates", func(t *testing.T) {
		got := evalString(t, `
def boom():
    raise ValueError("x")
d = {"a": 1}
try:
    d.get("zz", boom())
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("get with value key and default still works", func(t *testing.T) {
		got := evalString(t, `
d = {"a": 1}
str(d.get("a")) + "," + str(d.get("zz", "dflt"))
`)
		if got != "1,dflt" {
			t.Fatalf("result = %q, want %q", got, "1,dflt")
		}
	})
}

// The else clause of try/except must distinguish a real raise (skipped) from
// an exception VALUE flowing through the try body (runs) — the else condition
// now tests isRaised, not isException.
func TestTryElseDistinguishesRaiseFromValue(t *testing.T) {
	t.Run("else runs when body only binds a value exception", func(t *testing.T) {
		got := evalString(t, `
e = ValueError("v")
r = "none"
try:
    x = e
except ValueError:
    r = "caught"
else:
    r = "else-ran"
r
`)
		if got != "else-ran" {
			t.Fatalf("result = %q, want %q", got, "else-ran")
		}
	})

	t.Run("else skipped after a real raise", func(t *testing.T) {
		got := evalString(t, `
r = "none"
try:
    raise ValueError("v")
except ValueError:
    r = "caught"
else:
    r = "else-ran"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("else runs after a clean body", func(t *testing.T) {
		got := evalString(t, `
r = "none"
try:
    x = 1
except ValueError:
    r = "caught"
else:
    r = "else-ran"
r
`)
		if got != "else-ran" {
			t.Fatalf("result = %q, want %q", got, "else-ran")
		}
	})
}

// Re-throw shapes: a bare `raise` inside a handler re-raises so an OUTER
// try/except catches the original, and raising a new exception from a handler
// replaces the active exception.
func TestReThrowScenarios(t *testing.T) {
	t.Run("bare raise caught by outer try", func(t *testing.T) {
		got := evalString(t, `
r = "none"
try:
    try:
        raise ValueError("inner")
    except ValueError:
        raise
except ValueError as e:
    r = "outer:" + str(e)
r
`)
		if got != "outer:inner" {
			t.Fatalf("result = %q, want %q", got, "outer:inner")
		}
	})

	t.Run("raise new exception from handler", func(t *testing.T) {
		got := evalString(t, `
r = "none"
try:
    try:
        raise ValueError("first")
    except ValueError:
        raise TypeError("second")
except TypeError as e:
    r = "outer:" + str(e)
r
`)
		if got != "outer:second" {
			t.Fatalf("result = %q, want %q", got, "outer:second")
		}
	})
}

// Exception VALUE semantics (Python 3): constructed and caught exceptions are
// ordinary values — returnable from functions, usable in boolean operators,
// truthy — while `or` still propagates a raise from its left operand.
func TestExceptionValueSemantics(t *testing.T) {
	t.Run("function returning exception value", func(t *testing.T) {
		got := evalString(t, `
def make():
    return ValueError("made")
e = make()
"got:" + str(e)
`)
		if got != "got:made" {
			t.Fatalf("result = %q, want %q", got, "got:made")
		}
	})

	t.Run("value exceptions in boolean operators", func(t *testing.T) {
		got := evalString(t, `
e = ValueError("v")
str(e and 1) + "/" + str(1 or e)
`)
		if got != "1/1" {
			t.Fatalf("result = %q, want %q", got, "1/1")
		}
	})

	t.Run("caught exception is truthy and inspectable", func(t *testing.T) {
		got := evalString(t, `
try:
    raise ValueError("v")
except ValueError as ex:
    e = ex
r = "yes" if e else "no"
r
`)
		if got != "yes" {
			t.Fatalf("result = %q, want %q", got, "yes")
		}
	})

	t.Run("raise from or-left operand propagates", func(t *testing.T) {
		got := evalString(t, `
def boom():
    raise ValueError("x")
try:
    v = boom() or 1
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})
}

// An internal tagged error (e.g. ZeroDivisionError from `1 / 0`) that is
// caught by an except clause becomes a proper exception VALUE in the handler:
// str(e) yields the message and using e does not re-unwind.
func TestCaughtInternalErrorBecomesExceptionValue(t *testing.T) {
	got := evalString(t, `
r = "none"
try:
    x = 1 / 0
except ZeroDivisionError as e:
    r = "caught:" + str(e)
r
`)
	if got != "caught:division by zero" {
		t.Fatalf("result = %q, want %q", got, "caught:division by zero")
	}
}

// A raise from a user-defined __getitem__ must propagate out of the indexing
// expression and be catchable; the happy path must still return the item.
func TestDunderGetItemRaisePropagates(t *testing.T) {
	t.Run("__getitem__ raise is catchable", func(t *testing.T) {
		got := evalString(t, `
class Box:
    def __getitem__(self, i):
        raise ValueError("no-item")
b = Box()
try:
    v = b[0]
    r = "not-raised"
except ValueError as e:
    r = "caught:" + str(e)
r
`)
		if got != "caught:no-item" {
			t.Fatalf("result = %q, want %q", got, "caught:no-item")
		}
	})

	t.Run("__getitem__ value still returned", func(t *testing.T) {
		got := evalString(t, `
class Box:
    def __getitem__(self, i):
        return i * 10
Box()[3]
`)
		if got != "30" {
			t.Fatalf("result = %q, want %q", got, "30")
		}
	})
}

// next() over a user-defined iterator whose __next__ raises must propagate
// that raise; the happy path must still yield successive values.
func TestNextOverUserIteratorRaisePropagates(t *testing.T) {
	t.Run("raise from __next__ is catchable", func(t *testing.T) {
		got := evalString(t, `
class It:
    def __iter__(self):
        return self
    def __next__(self):
        raise ValueError("next-fail")
try:
    next(It())
    r = "not-raised"
except ValueError as e:
    r = "caught:" + str(e)
r
`)
		if got != "caught:next-fail" {
			t.Fatalf("result = %q, want %q", got, "caught:next-fail")
		}
	})

	t.Run("successive values still yielded", func(t *testing.T) {
		got := evalString(t, `
class It:
    def __init__(self):
        self.i = 0
    def __iter__(self):
        return self
    def __next__(self):
        self.i = self.i + 1
        return self.i * 2
it = It()
str(next(it)) + "," + str(next(it))
`)
		if got != "2,4" {
			t.Fatalf("result = %q, want %q", got, "2,4")
		}
	})
}

// A raise while evaluating the for-iterable must skip the loop's else clause
// (else runs only on clean completion, matching Python).
func TestForElseSkippedWhenIterableRaises(t *testing.T) {
	got := evalString(t, `
def boom():
    raise ValueError("x")
r = "none"
try:
    for i in boom():
        pass
    else:
        r = "else-ran"
except ValueError:
    r = "caught"
r
`)
	if got != "caught" {
		t.Fatalf("result = %q, want %q (else must not run on a raise)", got, "caught")
	}
}

// A raise from a user-defined __setitem__ must propagate out of the subscript
// assignment; the happy path must still store the value.
func TestDunderSetItemRaisePropagates(t *testing.T) {
	t.Run("__setitem__ raise is catchable", func(t *testing.T) {
		got := evalString(t, `
class Box:
    def __setitem__(self, i, v):
        raise ValueError("set-fail")
b = Box()
try:
    b[0] = 1
    r = "not-raised"
except ValueError as e:
    r = "caught:" + str(e)
r
`)
		if got != "caught:set-fail" {
			t.Fatalf("result = %q, want %q", got, "caught:set-fail")
		}
	})

	t.Run("__setitem__ still stores on the happy path", func(t *testing.T) {
		got := evalString(t, `
class Box:
    def __init__(self):
        self.data = {}
    def __setitem__(self, i, v):
        self.data[i] = v
    def __getitem__(self, i):
        return self.data[i]
b = Box()
b["k"] = 42
b["k"]
`)
		if got != "42" {
			t.Fatalf("result = %q, want %q", got, "42")
		}
	})
}

// Raises from property getters and setters must propagate through attribute
// access and assignment; the happy path must still work through both.
func TestPropertyRaisePropagates(t *testing.T) {
	t.Run("setter raise is catchable", func(t *testing.T) {
		got := evalString(t, `
class C:
    def __init__(self):
        self._x = 0
    @property
    def x(self):
        return self._x
    @x.setter
    def x(self, v):
        raise ValueError("set-fail")
c = C()
try:
    c.x = 5
    r = "not-raised"
except ValueError as e:
    r = "caught:" + str(e)
r
`)
		if got != "caught:set-fail" {
			t.Fatalf("result = %q, want %q", got, "caught:set-fail")
		}
	})

	t.Run("getter raise is catchable", func(t *testing.T) {
		got := evalString(t, `
class C:
    @property
    def x(self):
        raise ValueError("get-fail")
c = C()
try:
    v = c.x
    r = "not-raised"
except ValueError as e:
    r = "caught:" + str(e)
r
`)
		if got != "caught:get-fail" {
			t.Fatalf("result = %q, want %q", got, "caught:get-fail")
		}
	})

	t.Run("getter and setter happy path", func(t *testing.T) {
		got := evalString(t, `
class C:
    def __init__(self):
        self._x = 1
    @property
    def x(self):
        return self._x
    @x.setter
    def x(self, v):
        self._x = v * 10
c = C()
c.x = 3
c.x
`)
		if got != "30" {
			t.Fatalf("result = %q, want %q", got, "30")
		}
	})
}

// A raise inside a finally block replaces the in-flight exception (Python
// semantics); the finally's exception is what the outer except sees. The
// SystemExit variant is covered by finally_protected_test.go.
func TestFinallyRaiseReplacesInFlightException(t *testing.T) {
	got := evalString(t, `
r = "none"
try:
    try:
        raise ValueError("orig")
    finally:
        raise TypeError("fin")
except Exception as e:
    r = "caught:" + str(e)
r
`)
	if got != "caught:fin" {
		t.Fatalf("result = %q, want %q (finally's exception must win)", got, "caught:fin")
	}
}

// A break inside a finally block discards an in-flight exception from the try
// (Python semantics: break wins, the loop ends normally).
func TestBreakInFinallyDiscardsInFlightException(t *testing.T) {
	got := evalString(t, `
r = "none"
for i in [1]:
    try:
        raise ValueError("discarded")
    finally:
        break
r = "after-loop"
r
`)
	if got != "after-loop" {
		t.Fatalf("result = %q, want %q (break in finally must win)", got, "after-loop")
	}
}

// A decorator function that raises when applied must propagate the raise at
// definition time (the module-level path).
func TestDecoratorRaisePropagatesAtDefinition(t *testing.T) {
	got := evalString(t, `
def d(f):
    raise ValueError("apply-fail")
r = "none"
try:
    @d
    def f():
        return 1
    r = "defined"
except ValueError as e:
    r = "caught:" + str(e)
r
`)
	if got != "caught:apply-fail" {
		t.Fatalf("result = %q, want %q", got, "caught:apply-fail")
	}
}

// Raises from operator dunder methods (<, +, -) must propagate through the
// operator path; the happy paths must still compute.
func TestOperatorDunderRaisePropagates(t *testing.T) {
	cases := map[string]string{
		"__lt__ via <":  "v = B() < B()",
		"__add__ via +": "v = B() + B()",
		"__sub__ via -": "v = B() - B()",
		"__eq__ via ==": "v = B() == B()",
	}
	for name, stmt := range cases {
		t.Run(name, func(t *testing.T) {
			got := evalString(t, `
class B:
    def __lt__(self, other):
        raise ValueError("lt-fail")
    def __add__(self, other):
        raise ValueError("add-fail")
    def __sub__(self, other):
        raise ValueError("sub-fail")
    def __eq__(self, other):
        raise ValueError("eq-fail")
caught = "no"
try:
    `+stmt+`
except ValueError:
    caught = "yes"
caught
`)
			if got != "yes" {
				t.Fatalf("%s: result = %q, want %q", name, got, "yes")
			}
		})
	}

	t.Run("operator dunders still compute on happy path", func(t *testing.T) {
		got := evalString(t, `
class V:
    def __init__(self, n):
        self.n = n
    def __lt__(self, other):
        return self.n < other.n
    def __add__(self, other):
        return V(self.n + other.n)
    def __eq__(self, other):
        return self.n == other.n
r = str(V(1) < V(2)) + "," + str((V(1) + V(2)).n) + "," + str(V(1) == V(1))
r
`)
		if got != "True,3,True" {
			t.Fatalf("result = %q, want %q", got, "True,3,True")
		}
	})
}

// Raises from str(), len(), and repr() conversion paths must propagate; the
// happy paths must still convert.
func TestConversionDunderRaisePropagates(t *testing.T) {
	t.Run("str() with raising __str__", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        raise ValueError("str-fail")
try:
    s = str(B())
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("len() with raising __len__", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __len__(self):
        raise ValueError("len-fail")
try:
    n = len(B())
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("repr() with raising __repr__", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __repr__(self):
        raise ValueError("repr-fail")
try:
    s = repr(B())
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("conversions still work on happy path", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        return "B!"
    def __len__(self):
        return 7
    def __repr__(self):
        return "B(7)"
str(B()) + "," + str(len(B())) + "," + repr(B())
`)
		if got != "B!,7,B(7)" {
			t.Fatalf("result = %q, want %q", got, "B!,7,B(7)")
		}
	})
}

// A raise while evaluating a default argument must propagate at call time;
// the default must still be used when the argument is omitted and clean.
func TestDefaultArgumentRaisePropagatesAtCall(t *testing.T) {
	t.Run("raising default propagates", func(t *testing.T) {
		got := evalString(t, `
def boom():
    raise ValueError("defarg")
def f(x=boom()):
    return x
try:
    r = "f()=" + str(f())
except ValueError as e:
    r = "caught:" + str(e)
r
`)
		if got != "caught:defarg" {
			t.Fatalf("result = %q, want %q", got, "caught:defarg")
		}
	})

	t.Run("clean default still used", func(t *testing.T) {
		got := evalString(t, `
def f(x=10):
    return x * 2
str(f()) + "," + str(f(5))
`)
		if got != "20,10" {
			t.Fatalf("result = %q, want %q", got, "20,10")
		}
	})
}

// contextlib.suppress must suppress exactly the listed exception types and
// let others propagate (and run its body normally when nothing raises).
func TestContextlibSuppress(t *testing.T) {
	newP := func() *Scriptling {
		p := New()
		stdlibRegistered(t, p)
		return p
	}

	t.Run("matching type is suppressed", func(t *testing.T) {
		p := newP()
		_, err := p.Eval(`
import contextlib
r = "none"
with contextlib.suppress(ValueError):
    raise ValueError("sup")
r = "suppressed"
r
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		got, _ := p.GetVar("r")
		if got != "suppressed" {
			t.Fatalf("r = %v, want %q", got, "suppressed")
		}
	})

	t.Run("non-matching type propagates", func(t *testing.T) {
		p := newP()
		_, err := p.Eval(`
import contextlib
r = "none"
try:
    with contextlib.suppress(ValueError):
        raise TypeError("nope")
    r = "suppressed"
except TypeError:
    r = "caught"
r
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		got, _ := p.GetVar("r")
		if got != "caught" {
			t.Fatalf("r = %v, want %q", got, "caught")
		}
	})

	t.Run("body runs normally when nothing raises", func(t *testing.T) {
		p := newP()
		_, err := p.Eval(`
import contextlib
r = "none"
with contextlib.suppress(ValueError):
    r = "body-ran"
r
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		got, _ := p.GetVar("r")
		if got != "body-ran" {
			t.Fatalf("r = %v, want %q", got, "body-ran")
		}
	})
}

// stdlibRegistered registers the standard library on p (helper for tests that
// need import-able modules like contextlib or functools).
func stdlibRegistered(t *testing.T, p *Scriptling) {
	t.Helper()
	stdlib.RegisterAll(p)
}

// hash() must propagate a raise from __hash__ and return the dunder's value
// on the happy path.
func TestHashBuiltinDunderRaisePropagates(t *testing.T) {
	t.Run("raising __hash__ propagates", func(t *testing.T) {
		got := evalString(t, `
class C:
    def __hash__(self):
        raise ValueError("hash-fail")
try:
    v = hash(C())
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("__hash__ value returned", func(t *testing.T) {
		got := evalString(t, `
class C:
    def __hash__(self):
        return 7
hash(C())
`)
		if got != "7" {
			t.Fatalf("result = %q, want %q", got, "7")
		}
	})
}

// functools.reduce must propagate a raise from the combining function and
// compute on the happy path.
func TestFunctoolsReduceRaisePropagates(t *testing.T) {
	t.Run("raising combine function propagates", func(t *testing.T) {
		p := New()
		stdlibRegistered(t, p)
		result, err := p.Eval(`
import functools
def f(a, b):
    raise ValueError("red-fail")
r = "none"
try:
    v = functools.reduce(f, [1, 2])
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		if result.Inspect() != "caught" {
			t.Fatalf("result = %q, want %q", result.Inspect(), "caught")
		}
	})

	t.Run("reduce computes on happy path", func(t *testing.T) {
		p := New()
		stdlibRegistered(t, p)
		result, err := p.Eval(`
import functools
def add(a, b):
    return a + b
functools.reduce(add, [1, 2, 3], 0)
`)
		if err != nil {
			t.Fatalf("Eval failed: %v", err)
		}
		if result.Inspect() != "6" {
			t.Fatalf("result = %q, want %q", result.Inspect(), "6")
		}
	})
}

// A raise from super().__init__() must propagate through the subclass
// constructor; the happy-path super chain must still initialize.
func TestSuperInitRaisePropagates(t *testing.T) {
	t.Run("super().__init__ raise is catchable", func(t *testing.T) {
		got := evalString(t, `
class A:
    def __init__(self):
        raise ValueError("super-fail")
class B(A):
    def __init__(self):
        super().__init__()
try:
    b = B()
    r = "not-raised"
except ValueError as e:
    r = "caught:" + str(e)
r
`)
		if got != "caught:super-fail" {
			t.Fatalf("result = %q, want %q", got, "caught:super-fail")
		}
	})

	t.Run("super().__init__ chain works", func(t *testing.T) {
		got := evalString(t, `
class A:
    def __init__(self):
        self.x = 1
class B(A):
    def __init__(self):
        super().__init__()
        self.y = 2
b = B()
str(b.x) + "," + str(b.y)
`)
		if got != "1,2" {
			t.Fatalf("result = %q, want %q", got, "1,2")
		}
	})
}

// A continue inside finally discards an in-flight exception from the try and
// continues the loop (Python semantics).
func TestContinueInFinallyDiscardsInFlightException(t *testing.T) {
	got := evalString(t, `
r = "none"
n = 0
for i in [1, 2]:
    try:
        raise ValueError("discarded")
    finally:
        n = n + 1
        continue
r = "survived n=" + str(n)
r
`)
	if got != "survived n=2" {
		t.Fatalf("result = %q, want %q", got, "survived n=2")
	}
}

// Exception values are ordinary values: usable as dict keys and compared by
// identity (== on distinct instances is False), matching Python.
func TestExceptionValueIdentitySemantics(t *testing.T) {
	t.Run("exception value usable as dict key", func(t *testing.T) {
		got := evalString(t, `
e = ValueError("v")
d = {e: 1}
str(d[e])
`)
		if got != "1" {
			t.Fatalf("result = %q, want %q", got, "1")
		}
	})

	t.Run("distinct instances compare unequal", func(t *testing.T) {
		got := evalString(t, `
a = ValueError("x")
b = ValueError("x")
str(a == b) + "/" + str(a == a)
`)
		if got != "False/True" {
			t.Fatalf("result = %q, want %q", got, "False/True")
		}
	})
}

// Nested with statements whose __exit__ both raise: the outer exit's exception
// is the one that propagates (Python: each exit raise replaces the in-flight
// exception, inner first).
func TestNestedWithExitRaiseOrdering(t *testing.T) {
	got := evalString(t, `
class C:
    def __init__(self, name):
        self.name = name
    def __enter__(self):
        return self
    def __exit__(self, a, b, c):
        raise ValueError("exit-" + self.name)
r = "none"
try:
    with C("outer"):
        with C("inner"):
            pass
    r = "survived"
except ValueError as e:
    r = "caught:" + str(e)
r
`)
	if got != "caught:exit-outer" {
		t.Fatalf("result = %q, want %q", got, "caught:exit-outer")
	}
}

// A raise from __delitem__ must propagate out of del; the happy path must
// still delete.
func TestDunderDelItemRaisePropagates(t *testing.T) {
	t.Run("__delitem__ raise is catchable", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __delitem__(self, i):
        raise ValueError("del-fail")
b = B()
try:
    del b[0]
    r = "not-raised"
except ValueError as e:
    r = "caught:" + str(e)
r
`)
		if got != "caught:del-fail" {
			t.Fatalf("result = %q, want %q", got, "caught:del-fail")
		}
	})

	t.Run("__delitem__ still deletes on happy path", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __init__(self):
        self.data = {"k": 1}
    def __delitem__(self, i):
        del self.data[i]
b = B()
del b["k"]
str(len(b.data))
`)
		if got != "0" {
			t.Fatalf("result = %q, want %q", got, "0")
		}
	})
}

// A return inside a with body whose __exit__ then raises: the exit raise wins
// (Python: the exception raised in __exit__ replaces the return value).
func TestReturnInWithSupersededByExitRaise(t *testing.T) {
	got := evalString(t, `
class C:
    def __enter__(self):
        return self
    def __exit__(self, a, b, c):
        raise ValueError("exit-fail")
def f():
    with C():
        return "returned"
try:
    r = f()
except ValueError as e:
    r = "caught:" + str(e)
r
`)
	if got != "caught:exit-fail" {
		t.Fatalf("result = %q, want %q", got, "caught:exit-fail")
	}
}

// Python's bare `raise SomeExceptionClass` instantiates the exception with no
// arguments. Builtin exception constructors are recognized by name; other
// operands (non-exception builtins, plain values) still error, and explicit
// constructor calls are unaffected.
func TestRaiseBareExceptionClass(t *testing.T) {
	t.Run("bare constructor is caught with empty message", func(t *testing.T) {
		got := evalString(t, `
r = "none"
try:
    raise ValueError
    r = "not-raised"
except ValueError as e:
    r = "caught msg=[" + str(e) + "]"
r
`)
		if got != "caught msg=[]" {
			t.Fatalf("result = %q, want %q", got, "caught msg=[]")
		}
	})

	t.Run("bare StopIteration ends iteration", func(t *testing.T) {
		got := evalString(t, `
class Range:
    def __init__(self, n):
        self.i = 0
        self.n = n
    def __iter__(self):
        return self
    def __next__(self):
        if self.i >= self.n:
            raise StopIteration
        self.i = self.i + 1
        return self.i
str(list(Range(3)))
`)
		if got != "[1, 2, 3]" {
			t.Fatalf("result = %q, want %q", got, "[1, 2, 3]")
		}
	})

	t.Run("explicit constructor call unchanged", func(t *testing.T) {
		got := evalString(t, `
try:
    raise ValueError("msg")
    r = "no"
except ValueError as e:
    r = str(e)
r
`)
		if got != "msg" {
			t.Fatalf("result = %q, want %q", got, "msg")
		}
	})

	t.Run("non-exception operands still error", func(t *testing.T) {
		p := New()
		if _, err := p.Eval(`raise 42`); err == nil {
			t.Fatal("expected raise 42 to error")
		}
		p2 := New()
		if _, err := p2.Eval(`raise len`); err == nil {
			t.Fatal("expected raise len to error")
		}
	})

	t.Run("raised instance values unaffected", func(t *testing.T) {
		got := evalString(t, `
myexc = ValueError("stored")
try:
    raise myexc
    r = "no"
except ValueError as e:
    r = str(e)
r
`)
		if got != "stored" {
			t.Fatalf("result = %q, want %q", got, "stored")
		}
	})
}
