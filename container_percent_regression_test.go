package scriptling

import (
	"strings"
	"testing"
	"time"
)

// Regression tests for container operations honouring user-defined __eq__ and
// __hash__ (including propagating their raises) and for % formatting
// dispatching to __str__/__repr__ on instances. Previously these paths used
// structural/reference equality and unchecked hashing, so a working __eq__ was
// ignored by list/tuple membership methods and a raising __eq__/__hash__
// produced silent wrong answers (False, 0, "not found") or silently stored
// entries under a fallback hash.

// List membership and index/count/remove must use __eq__ for instances.
func TestListMethodsHonorEq(t *testing.T) {
	t.Run("working __eq__ matches", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __init__(self, v):
        self.v = v
    def __eq__(self, other):
        return self.v == other.v
l = [B(1), B(2)]
str(B(1) in l) + "," + str(l.count(B(1))) + "," + str(l.index(B(2)))
`)
		if got != "True,1,1" {
			t.Fatalf("result = %q, want %q", got, "True,1,1")
		}
	})

	t.Run("raising __eq__ propagates from in/count/index/remove", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __eq__(self, other):
        raise ValueError("eq-fail")
l = [B()]
r1 = "none"
try:
    v = B() in l
    r1 = "in-not-raised"
except ValueError:
    r1 = "in-caught"
r2 = "none"
try:
    n = l.count(B())
    r2 = "count-not-raised"
except ValueError:
    r2 = "count-caught"
r3 = "none"
try:
    i = l.index(B())
    r3 = "index-not-raised"
except ValueError:
    r3 = "index-caught"
r4 = "none"
try:
    l.remove(B())
    r4 = "remove-not-raised"
except ValueError:
    r4 = "remove-caught"
r1 + "/" + r2 + "/" + r3 + "/" + r4
`)
		if got != "in-caught/count-caught/index-caught/remove-caught" {
			t.Fatalf("result = %q, want all four caught", got)
		}
	})
}

// Tuple index/count must use __eq__ for instances.
func TestTupleMethodsHonorEq(t *testing.T) {
	t.Run("working __eq__ matches", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __init__(self, v):
        self.v = v
    def __eq__(self, other):
        return self.v == other.v
t = (B(1), B(2))
str(t.count(B(1))) + "," + str(t.index(B(2)))
`)
		if got != "1,1" {
			t.Fatalf("result = %q, want %q", got, "1,1")
		}
	})

	t.Run("raising __eq__ propagates", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __eq__(self, other):
        raise ValueError("eq-fail")
t = (B(),)
r1 = "none"
try:
    n = t.count(B())
    r1 = "count-not-raised"
except ValueError:
    r1 = "count-caught"
r2 = "none"
try:
    i = t.index(B())
    r2 = "index-not-raised"
except ValueError:
    r2 = "index-caught"
r1 + "/" + r2
`)
		if got != "count-caught/index-caught" {
			t.Fatalf("result = %q, want both caught", got)
		}
	})
}

// The in/not-in operators must dispatch instance comparison through __eq__
// and instance hashing through __hash__, propagating raises.
func TestInOperatorHonorsEqAndHash(t *testing.T) {
	t.Run("working __eq__ membership", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __init__(self, v):
        self.v = v
    def __eq__(self, other):
        return self.v == other.v
str(B(2) in [B(1), B(2)]) + "," + str(B(9) in [B(1)])
`)
		if got != "True,False" {
			t.Fatalf("result = %q, want %q", got, "True,False")
		}
	})

	t.Run("raising __eq__ propagates", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __eq__(self, other):
        raise ValueError("eq-fail")
try:
    v = B() in [B()]
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("raising __hash__ propagates through in and not-in", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __hash__(self):
        raise ValueError("hash-fail")
d = {}
r1 = "none"
try:
    v = B() in d
    r1 = "in-not-raised"
except ValueError:
    r1 = "in-caught"
r2 = "none"
try:
    v = B() not in d
    r2 = "notin-not-raised"
except ValueError:
    r2 = "notin-caught"
r1 + "/" + r2
`)
		if got != "in-caught/notin-caught" {
			t.Fatalf("result = %q, want %q", got, "in-caught/notin-caught")
		}
	})

	t.Run("structural membership for builtin types unchanged", func(t *testing.T) {
		got := evalString(t, `
str([3, 4] in [[1, 2], [3, 4]]) + "," + str([9] in [[1]])
`)
		if got != "True,False" {
			t.Fatalf("result = %q, want %q", got, "True,False")
		}
	})
}

// Every dict operation must hash keys through the checked path: a raising
// __hash__ propagates instead of producing KeyError/False or silently storing
// the entry. The happy path with a working __hash__ must be unchanged.
func TestDictOperationsPropagateHashRaise(t *testing.T) {
	t.Run("raising __hash__ propagates from in and read", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __hash__(self):
        raise ValueError("hash-fail")
d = {}
b = B()
r1 = "none"
try:
    v = b in d
    r1 = "in-not-raised"
except ValueError:
    r1 = "in-caught"
r2 = "none"
try:
    v = d[b]
    r2 = "read-not-raised"
except ValueError:
    r2 = "read-caught"
except KeyError:
    r2 = "read-keyerror"
r1 + "/" + r2
`)
		if got != "in-caught/read-caught" {
			t.Fatalf("result = %q, want %q", got, "in-caught/read-caught")
		}
	})

	t.Run("setdefault does not store on raising __hash__", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __hash__(self):
        raise ValueError("hash-fail")
d = {}
r = "none"
try:
    d.setdefault(B(), 1)
    r = "stored len=" + str(len(d))
except ValueError:
    r = "caught len=" + str(len(d))
r
`)
		if got != "caught len=0" {
			t.Fatalf("result = %q, want %q", got, "caught len=0")
		}
	})

	t.Run("assign and del propagate the raise", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __hash__(self):
        raise ValueError("hash-fail")
d = {}
b = B()
r1 = "none"
try:
    d[b] = 1
    r1 = "assign-not-raised"
except ValueError:
    r1 = "assign-caught"
r2 = "none"
try:
    del d[b]
    r2 = "del-not-raised"
except ValueError:
    r2 = "del-caught"
except KeyError:
    r2 = "del-keyerror"
r1 + "/" + r2 + " len=" + str(len(d))
`)
		if got != "assign-caught/del-caught len=0" {
			t.Fatalf("result = %q, want %q", got, "assign-caught/del-caught len=0")
		}
	})

	t.Run("get and pop propagate the raise", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __hash__(self):
        raise ValueError("hash-fail")
d = {}
b = B()
r1 = "none"
try:
    d.get(b)
    r1 = "get-not-raised"
except ValueError:
    r1 = "get-caught"
r2 = "none"
try:
    d.pop(b, None)
    r2 = "pop-not-raised"
except ValueError:
    r2 = "pop-caught"
r1 + "/" + r2
`)
		if got != "get-caught/pop-caught" {
			t.Fatalf("result = %q, want %q", got, "get-caught/pop-caught")
		}
	})

	t.Run("update and fromkeys propagate the raise", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __hash__(self):
        raise ValueError("hash-fail")
b = B()
r1 = "none"
try:
    {}.update([[b, 1]])
    r1 = "update-not-raised"
except ValueError:
    r1 = "update-caught"
r2 = "none"
try:
    {}.fromkeys([b])
    r2 = "fromkeys-not-raised"
except ValueError:
    r2 = "fromkeys-caught"
r1 + "/" + r2
`)
		if got != "update-caught/fromkeys-caught" {
			t.Fatalf("result = %q, want %q", got, "update-caught/fromkeys-caught")
		}
	})

	t.Run("working __hash__ round-trips as a key", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __init__(self, v):
        self.v = v
    def __hash__(self):
        return self.v
d = {}
k = B(7)
d[k] = "hit"
str(d[k]) + "," + str(k in d) + "," + str(d.get(k))
`)
		if got != "hit,True,hit" {
			t.Fatalf("result = %q, want %q", got, "hit,True,hit")
		}
	})
}

// Set remove/discard must hash through the checked path.
func TestSetOperationsPropagateHashRaise(t *testing.T) {
	t.Run("raising __hash__ propagates", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __hash__(self):
        raise ValueError("hash-fail")
s = set([1])
b = B()
r1 = "none"
try:
    s.remove(b)
    r1 = "remove-not-raised"
except ValueError:
    r1 = "remove-caught"
r2 = "none"
try:
    s.discard(b)
    r2 = "discard-not-raised"
except ValueError:
    r2 = "discard-caught"
r1 + "/" + r2
`)
		if got != "remove-caught/discard-caught" {
			t.Fatalf("result = %q, want %q", got, "remove-caught/discard-caught")
		}
	})

	t.Run("working __hash__ discard still works", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __init__(self, v):
        self.v = v
    def __hash__(self):
        return self.v
s = set([B(1), B(2)])
s.discard(B(1))
len(s)
`)
		if got != "1" {
			t.Fatalf("result = %q, want %q", got, "1")
		}
	})
}

// %s must convert instances with __str__ (raise propagating) and %r with
// __repr__, falling back to __str__ then the default; plain values unchanged.
func TestPercentFormatHonorsDunders(t *testing.T) {
	t.Run("%s uses __str__", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        return "B-str"
"%s" % B()
`)
		if got != "B-str" {
			t.Fatalf("result = %q, want %q", got, "B-str")
		}
	})

	t.Run("%s propagates raising __str__", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        raise ValueError("str-fail")
try:
    s = "%s" % B()
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("%r uses __repr__ then __str__", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __repr__(self):
        return "B(repr)"
class C:
    def __str__(self):
        return "C(str)"
"%r|%r" % (B(), C())
`)
		if got != "B(repr)|C(str)" {
			t.Fatalf("result = %q, want %q", got, "B(repr)|C(str)")
		}
	})

	t.Run("plain values format unchanged", func(t *testing.T) {
		got := evalString(t, `
"%s-%d-%s" % ("x", 7, [1, 2])
`)
		if got != "x-7-[1, 2]" {
			t.Fatalf("result = %q, want %q", got, "x-7-[1, 2]")
		}
	})
}

// Reading a non-string index from an instance without __getitem__ raises a
// TypeError ("not subscriptable"), catchable as TypeError.
func TestInstanceNotSubscriptableTypeError(t *testing.T) {
	got := evalString(t, `
class C:
    pass
c = C()
try:
    v = c[0]
    r = "not-raised"
except TypeError as e:
    r = "caught:" + str(e)
r
`)
	want := "caught:'C' object is not subscriptable"
	if got != want {
		t.Fatalf("result = %q, want %q", got, want)
	}
}

// Mapping-pattern keys are value expressions evaluated at match time in the
// enclosing scope (PEP 634): variables and calls are visible, and a raising
// key expression propagates instead of reporting "identifier not found".
func TestMatchPatternKeysEvaluateInScope(t *testing.T) {
	t.Run("variable key matches", func(t *testing.T) {
		got := evalString(t, `
KEY = "k"
d = {"k": 42}
match d:
    case {KEY: v}:
        r = "matched v=" + str(v)
    case _:
        r = "no-match"
r
`)
		if got != "matched v=42" {
			t.Fatalf("result = %q, want %q", got, "matched v=42")
		}
	})

	t.Run("literal keys still match", func(t *testing.T) {
		got := evalString(t, `
d = {"k": 1}
match d:
    case {"k": v}:
        r = "matched"
    case _:
        r = "no-match"
r
`)
		if got != "matched" {
			t.Fatalf("result = %q, want %q", got, "matched")
		}
	})

	t.Run("call key matches", func(t *testing.T) {
		got := evalString(t, `
def make_key():
    return "k"
d = {"k": 7}
match d:
    case {make_key(): v}:
        r = "matched v=" + str(v)
    case _:
        r = "no-match"
r
`)
		if got != "matched v=7" {
			t.Fatalf("result = %q, want %q", got, "matched v=7")
		}
	})

	t.Run("raising key expression propagates", func(t *testing.T) {
		got := evalString(t, `
def boom():
    raise ValueError("key-fail")
d = {"k": 1}
try:
    match d:
        case {boom(): v}:
            r = "matched"
        case _:
            r = "no-match"
    r = r + "/not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("missing key does not match", func(t *testing.T) {
		got := evalString(t, `
K = "zz"
d = {"k": 1}
match d:
    case {K: v}:
        r = "matched"
    case _:
        r = "no-match"
r
`)
		if got != "no-match" {
			t.Fatalf("result = %q, want %q", got, "no-match")
		}
	})
}

// f-strings format exceptions as their message (str(e)) like Python, not as
// "EXCEPTION: msg"; format specs still apply, and instances keep __str__.
func TestFStringFormatsExceptionAsMessage(t *testing.T) {
	t.Run("caught exception renders message", func(t *testing.T) {
		got := evalString(t, `
try:
    raise ValueError("bang")
except ValueError as e:
    r = f"[{e}]"
r
`)
		if got != "[bang]" {
			t.Fatalf("result = %q, want %q", got, "[bang]")
		}
	})

	t.Run("format spec applies to message", func(t *testing.T) {
		got := evalString(t, `
e = ValueError("stored")
f"[{e:>10}]"
`)
		if got != "[    stored]" {
			t.Fatalf("result = %q, want %q", got, "[    stored]")
		}
	})

	t.Run("instance __str__ still honoured", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        return "B-str"
f"{B()}"
`)
		if got != "B-str" {
			t.Fatalf("result = %q, want %q", got, "B-str")
		}
	})
}

// str.format converts arguments the way str() does: instances dispatch
// __str__ (raise propagating), exceptions render their message, and plain
// positional formatting is unchanged.
func TestStrFormatDispatchesStr(t *testing.T) {
	t.Run("instance __str__ used positionally and indexed", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        return "B-str"
"{}|{0}".format(B())
`)
		if got != "B-str|B-str" {
			t.Fatalf("result = %q, want %q", got, "B-str|B-str")
		}
	})

	t.Run("raising __str__ propagates", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        raise ValueError("str-fail")
try:
    s = "{}".format(B())
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("exception argument renders message", func(t *testing.T) {
		got := evalString(t, `
try:
    raise ValueError("bang")
except ValueError as e:
    r = "[{}]".format(e)
r
`)
		if got != "[bang]" {
			t.Fatalf("result = %q, want %q", got, "[bang]")
		}
	})

	t.Run("plain values unchanged", func(t *testing.T) {
		got := evalString(t, `"{}-{}".format(1, "a")`)
		if got != "1-a" {
			t.Fatalf("result = %q, want %q", got, "1-a")
		}
	})
}

// When a with body raises and __exit__ returns an instance whose __bool__
// raises, that raise propagates (Python evaluates the exit result's
// truthiness); truthy exit still suppresses, falsy still propagates.
func TestWithSuppressionTruthinessPropagatesRaise(t *testing.T) {
	t.Run("raising __bool__ from exit result propagates", func(t *testing.T) {
		got := evalString(t, `
class T:
    def __bool__(self):
        raise ValueError("bool-fail")
class C:
    def __enter__(self):
        return self
    def __exit__(self, a, b, c):
        return T()
try:
    with C():
        raise TypeError("body-fail")
    r = "no-raise"
except ValueError as e:
    r = "bool-caught:" + str(e)
except TypeError:
    r = "body-propagated"
r
`)
		if got != "bool-caught:bool-fail" {
			t.Fatalf("result = %q, want %q", got, "bool-caught:bool-fail")
		}
	})

	t.Run("truthy exit still suppresses body raise", func(t *testing.T) {
		got := evalString(t, `
class C:
    def __enter__(self):
        return self
    def __exit__(self, a, b, c):
        return True
with C():
    raise ValueError("body")
"survived"
`)
		if got != "survived" {
			t.Fatalf("result = %q, want %q", got, "survived")
		}
	})

	t.Run("falsy exit still propagates body raise", func(t *testing.T) {
		got := evalString(t, `
class C:
    def __enter__(self):
        return self
    def __exit__(self, a, b, c):
        return False
try:
    with C():
        raise ValueError("body")
    r = "no-raise"
except ValueError:
    r = "propagated"
r
`)
		if got != "propagated" {
			t.Fatalf("result = %q, want %q", got, "propagated")
		}
	})
}

// str.format supports the Python field syntax: {} auto-numbering, {0} explicit
// indexes, {name} keyword fields, {{ }} escapes and :format specs. Errors are
// raised for missing arguments, unknown names, bad indexes and stray braces.
func TestStrFormatNamedFieldsAndSpecs(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"named field", `"{x}".format(x=5)`, "5"},
		{"named and positional", `"{0}-{x}-{1}".format("a", "b", x="X")`, "a-X-b"},
		{"auto field", `"<{}>".format(7)`, "<7>"},
		{"brace escapes", `"{{}}".format()`, "{}"},
		{"escapes with fields", `"{{{}}}".format(1)`, "{1}"},
		{"spec on named", `"{x:>6}".format(x="ab")`, "    ab"},
		{"spec on auto", `"{:d}".format(42)`, "42"},
		{"float spec", `"{:.2f}".format(3.14159)`, "3.14"},
		{"string args unquoted", `"[{}]".format("s")`, "[s]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evalString(t, tc.src)
			if got != tc.want {
				t.Fatalf("result = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("instance __str__ through named field", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        return "B!"
"{x}".format(x=B())
`)
		if got != "B!" {
			t.Fatalf("result = %q, want %q", got, "B!")
		}
	})

	t.Run("raising __str__ propagates", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        raise ValueError("s")
try:
    "{}".format(B())
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("exception argument renders message", func(t *testing.T) {
		got := evalString(t, `
try:
    raise ValueError("boom")
except ValueError as e:
    r = "[{}]".format(e)
r
`)
		if got != "[boom]" {
			t.Fatalf("result = %q, want %q", got, "[boom]")
		}
	})

	errCases := []struct {
		name string
		src  string
		want string
	}{
		{"missing keyword", `"{x}".format(y=1)`, "format field 'x' has no matching keyword argument"},
		{"too few arguments", `"{} {}".format(1)`, "not enough arguments for format string"},
		{"index out of range", `"{5}".format(1)`, "format index 5 out of range"},
		{"single open brace", `"{".format(1)`, "single '{' in format string"},
		{"single close brace", `"}".format(1)`, "single '}' in format string"},
	}
	for _, tc := range errCases {
		t.Run("error: "+tc.name, func(t *testing.T) {
			p := New()
			_, err := p.Eval(tc.src)
			if err == nil {
				t.Fatalf("expected error for %q", tc.src)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %q, want it to contain %q", err.Error(), tc.want)
			}
		})
	}
}

// Comparisons reflect: when the left operand defines no __lt__ (or mirrored
// dunder), the right operand's __gt__ (etc.) is tried, for sorted(), min/max
// and the comparison operators alike. Left-side dunders take precedence.
func TestReflectedComparisons(t *testing.T) {
	t.Run("sorted with only __gt__", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __init__(self, v):
        self.v = v
    def __gt__(self, o):
        return self.v > o.v
l = sorted([B(3), B(1), B(2)])
str(l[0].v) + "," + str(l[1].v) + "," + str(l[2].v)
`)
		if got != "1,2,3" {
			t.Fatalf("result = %q, want %q", got, "1,2,3")
		}
	})

	t.Run("min with only __gt__", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __init__(self, v):
        self.v = v
    def __gt__(self, o):
        return self.v > o.v
min(B(5), B(2)).v
`)
		if got != "2" {
			t.Fatalf("result = %q, want %q", got, "2")
		}
	})

	t.Run("< operator reflects to __gt__", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __init__(self, v):
        self.v = v
    def __gt__(self, o):
        return self.v > o.v
str(B(1) < B(2)) + "," + str(B(2) < B(1))
`)
		if got != "True,False" {
			t.Fatalf("result = %q, want %q", got, "True,False")
		}
	})

	t.Run("== reflects to the right operand's __eq__", func(t *testing.T) {
		got := evalString(t, `
class Left:
    pass
class Right:
    def __eq__(self, o):
        return True
str(Left() == Right()) + "," + str(Right() == Left())
`)
		if got != "True,True" {
			t.Fatalf("result = %q, want %q", got, "True,True")
		}
	})

	t.Run("raising reflected __gt__ propagates through sorted", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __gt__(self, o):
        raise ValueError("gt-fail")
try:
    sorted([B(), B()])
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("left-side __lt__ takes precedence over reflection", func(t *testing.T) {
		got := evalString(t, `
class L:
    def __init__(self, v):
        self.v = v
    def __lt__(self, o):
        return self.v < o.v
class R:
    def __init__(self, v):
        self.v = v
    def __gt__(self, o):
        return True
str(L(9) < R(1))
`)
		if got != "False" {
			t.Fatalf("result = %q, want %q (left __lt__ must win over reflected __gt__)", got, "False")
		}
	})
}

// Builtins that accept iterables must accept class instances with
// __iter__/__next__ (the full iterator protocol, as for-loops do), and a
// raise from the protocol must propagate instead of hanging or being
// swallowed. Previously every materializing builtin rejected instances with
// "expected iterable, got INSTANCE".
func TestBuiltinsAcceptInstanceIterables(t *testing.T) {
	rangeCls := `
class Range:
    def __init__(self, n):
        self.i = 0
        self.n = n
    def __iter__(self):
        return self
    def __next__(self):
        if self.i >= self.n:
            raise StopIteration()
        self.i = self.i + 1
        return self.i
`
	t.Run("materializing builtins consume instance iterators", func(t *testing.T) {
		got := evalString(t, rangeCls+`
parts = []
parts.append(str(list(Range(3))))
parts.append(str(tuple(Range(3))))
parts.append(str(len(set(Range(3)))))
parts.append(str(sorted(Range(3), reverse=True)))
parts.append(str(list(map(lambda x: x * 10, Range(3)))))
parts.append(str(sum(Range(3))))
parts.append(str(min(Range(3))) + "," + str(max(Range(3))))
parts.append(str(list(enumerate(Range(2)))))
parts.append(str(list(zip(Range(2), Range(3)))))
",".join(parts)
`)
		want := "[1, 2, 3],(1, 2, 3),3,[3, 2, 1],[10, 20, 30],6,1,3,[(0, 1), (1, 2)],[(1, 1), (2, 2)]"
		if got != want {
			t.Fatalf("result = %q, want %q", got, want)
		}
	})

	t.Run("unpacking an instance iterator", func(t *testing.T) {
		got := evalString(t, rangeCls+`
a, b = Range(2)
str(a) + "," + str(b)
`)
		if got != "1,2" {
			t.Fatalf("result = %q, want %q", got, "1,2")
		}
	})

	t.Run("dict from an instance iterator of pairs", func(t *testing.T) {
		got := evalString(t, `
class Pairs:
    def __init__(self):
        self.i = 0
    def __iter__(self):
        return self
    def __next__(self):
        if self.i >= 2:
            raise StopIteration()
        self.i = self.i + 1
        return (self.i, self.i * 10)
str(dict(Pairs()))
`)
		if got != "{1: 10, 2: 20}" {
			t.Fatalf("result = %q, want %q", got, "{1: 10, 2: 20}")
		}
	})

	t.Run("raise from the iterator protocol propagates", func(t *testing.T) {
		badIter := `
class BadIter:
    def __iter__(self):
        return self
    def __next__(self):
        raise ValueError("next-fail")
`
		cases := map[string]string{
			"list":      `l = list(BadIter())`,
			"map":       `l = list(map(str, BadIter()))`,
			"sorted":    `l = sorted(BadIter())`,
			"sum":       `v = sum(BadIter())`,
			"enumerate": `l = list(enumerate(BadIter()))`,
			"zip":       `l = list(zip(BadIter(), [1]))`,
			"join":      `s = ",".join(BadIter())`,
			"unpack":    `a, b = BadIter()`,
			"dict":      `d = dict(BadIter())`,
		}
		for name, stmt := range cases {
			t.Run(name, func(t *testing.T) {
				got := evalString(t, badIter+`
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
	})

	t.Run("plain container iterables unchanged", func(t *testing.T) {
		got := evalString(t, `
str(list("abc")) + "," + str(sorted([3, 1, 2])) + "," + str(list(zip([1, 2], "ab")))
`)
		if got != "[a, b, c],[1, 2, 3],[(1, a), (2, b)]" {
			t.Fatalf("result = %q, want %q", got, "[a, b, c],[1, 2, 3],[(1, a), (2, b)]")
		}
	})
}

// !r/!s conversion flags work in both f-strings and str.format: !r uses repr
// semantics (with Python's str→repr fallback and quoted strings), !s uses str
// semantics, and raises from the dunders propagate.
func TestFormatConversionFlags(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"f-string !r quotes strings", "\nx = \"s\"\nf\"{x!r}\"", "\"s\""},
		{"format !r quotes strings", "\nx = \"s\"\n\"{x!r}\".format(x=x)", "\"s\""},
		{"!r on int unchanged", "\nf\"{42!r}\"", "42"},
		{"!r uses __repr__", `
class B:
    def __repr__(self):
        return "R"
    def __str__(self):
        return "S"
f"[{B()!r}|{B()!s}]"
`, "[R|S]"},
		{"!s falls back to __repr__", `
class B:
    def __repr__(self):
        return "R"
"[{0!s}]".format(B())
`, "[R]"},
		{"str() falls back to __repr__", `
class B:
    def __repr__(self):
        return "R"
str(B())
`, "R"},
		{"f-string falls back to __repr__", `
class B:
    def __repr__(self):
        return "R"
f"[{B()}]"
`, "[R]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evalString(t, tc.src)
			if got != tc.want {
				t.Fatalf("result = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("raising __repr__ propagates through !r", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __repr__(self):
        raise ValueError("r")
try:
    s = f"{B()!r}"
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("unknown conversion errors", func(t *testing.T) {
		p := New()
		if _, err := p.Eval(`"{x!z}".format(x=1)`); err == nil {
			t.Fatal("expected unknown conversion to error")
		}
	})
}

// Nested spec fields ({x:>{w}}) resolve in both str.format (against
// positional and keyword arguments) and f-strings (against the enclosing
// scope), with dynamic widths and fill patterns.
func TestNestedFormatSpecFields(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"format nested by name", `"[{x:>{w}}]".format(x="ab", w=8)`, "[      ab]"},
		{"format nested by index", `"[{0:>{1}}]".format("ab", 8)`, "[      ab]"},
		{"f-string nested width", "\nw = 8\nf\"[{4:{w}}]\"", "[       4]"},
		{"f-string nested fill", "\nw = 5\nf\"[{3:0{w}d}]\"", "[00003]"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := evalString(t, tc.src)
			if got != tc.want {
				t.Fatalf("result = %q, want %q", got, tc.want)
			}
		})
	}

	t.Run("missing nested field errors", func(t *testing.T) {
		p := New()
		if _, err := p.Eval(`"{x:>{w}}".format(x="a")`); err == nil {
			t.Fatal("expected missing nested keyword to error")
		}
		p2 := New()
		if _, err := p2.Eval("f\"{3:{missing_w}}\""); err == nil {
			t.Fatal("expected undefined nested name to error")
		}
	})
}

// Materializing builtins over a non-terminating user iterator must remain
// cancellable by context timeout, exactly like a for-loop over it.
func TestTimeoutCancelsMaterialization(t *testing.T) {
	forever := `
class Forever:
    def __init__(self):
        self.i = 0
    def __iter__(self):
        return self
    def __next__(self):
        self.i = self.i + 1
        return self.i
`
	scripts := map[string]string{
		"list":     forever + "\nl = list(Forever())\n",
		"sorted":   forever + "\ns = sorted(Forever())\n",
		"for-loop": forever + "\nn = 0\nfor x in Forever():\n    n = n + 1\n",
	}
	for name, src := range scripts {
		t.Run(name, func(t *testing.T) {
			p := New()
			start := time.Now()
			result, err := p.EvalWithTimeout(300*time.Millisecond, src)
			elapsed := time.Since(start)
			if elapsed > 2*time.Second {
				t.Fatalf("%s ran %v; not cancellable", name, elapsed)
			}
			if err == nil && (result == nil || !strings.Contains(result.Inspect(), "timeout")) {
				t.Fatalf("%s: expected timeout, got err=%v result=%v", name, err, result)
			}
		})
	}
}

// str.join converts elements with str() semantics: instances dispatch __str__
// (falling back to __repr__), raises propagate, and plain strings are used
// as-is.
func TestJoinDispatchesStr(t *testing.T) {
	t.Run("instance __str__ used", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        return "B"
",".join(["a", B()])
`)
		if got != "a,B" {
			t.Fatalf("result = %q, want %q", got, "a,B")
		}
	})

	t.Run("repr-only class falls back", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __repr__(self):
        return "R"
",".join(["a", B()])
`)
		if got != "a,R" {
			t.Fatalf("result = %q, want %q", got, "a,R")
		}
	})

	t.Run("raising __str__ propagates", func(t *testing.T) {
		got := evalString(t, `
class B:
    def __str__(self):
        raise ValueError("s")
try:
    s = ",".join(["a", B()])
    r = "not-raised"
except ValueError:
    r = "caught"
r
`)
		if got != "caught" {
			t.Fatalf("result = %q, want %q", got, "caught")
		}
	})

	t.Run("plain elements unchanged", func(t *testing.T) {
		got := evalString(t, `",".join(["a", "b"])`)
		if got != "a,b" {
			t.Fatalf("result = %q, want %q", got, "a,b")
		}
	})
}
