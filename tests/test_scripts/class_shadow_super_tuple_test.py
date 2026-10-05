# Class-attribute shadowing, starred tuple displays, super() fallbacks,
# UUID namespace constants, bool coercion and %g precision.

# --- subclass class attributes shadow base attributes ---
class A:
    kind = "base"
    n = 1
    def hello(self):
        return "A.hello"

class C(A):
    kind = "cat"
    m = 2
    def hello(self):
        return "C.hello"

assert C.kind == "cat" and C().kind == "cat" and A.kind == "base"
assert C.n == 1 and C.m == 2 and C().hello() == "C.hello"

class D(A):
    hello = "attr-shadows-method"
assert D.hello == "attr-shadows-method" and D().hello == "attr-shadows-method"

class E(C):
    kind = "kitten"
assert E.kind == "kitten" and C.kind == "cat" and A.kind == "base"

class F(A):
    kind = A.kind + "!"
assert F.kind == "base!"

class G(A):
    def kind(self):
        return "method"
assert G().kind() == "method"

class H(A):
    pass
assert H.kind == "base"
h = H()
h.kind = "inst"
assert h.kind == "inst" and H.kind == "base" and A.kind == "base"

# --- starred tuple displays ---
t = (1, 2)
assert (*t,) == (1, 2)
assert (1, *[2, 3], 4) == (1, 2, 3, 4)
assert (*t, *t) == (1, 2, 1, 2)
assert (*"ab",) == ("a", "b")
assert (*range(3), 9) == (0, 1, 2, 9)
assert (*[],) == ()
try:
    (*5,)
    assert False, "expected TypeError"
except TypeError:
    pass

# --- super() falls back to object's methods ---
class Plain:
    pass

class Child(Plain):
    def __init__(self):
        super().__init__()
        self.x = 1
assert Child().x == 1

class NoBase:
    def __init__(self, v):
        super().__init__()
        self.v = v
assert NoBase(2).v == 2

class Leaf(Child):
    def __init__(self):
        super().__init__()
        self.y = 2
leaf = Leaf()
assert leaf.x == 1 and leaf.y == 2

class Strict(Plain):
    def __init__(self):
        super().__init__(1)
try:
    Strict()
    assert False, "expected TypeError"
except TypeError:
    pass

class Eq(Plain):
    def __eq__(self, other):
        return super().__eq__(other)
e1 = Eq()
assert e1 == e1 and not (e1 == Eq())

# --- UUID namespace constants are UUID objects ---
import uuid
ns = "6ba7b810-9dad-11d1-80b4-00c04fd430c8"
assert uuid.NAMESPACE_DNS == uuid.UUID(ns)
assert uuid.UUID(ns) == uuid.NAMESPACE_DNS
assert uuid.NAMESPACE_DNS != uuid.NAMESPACE_URL
assert str(uuid.NAMESPACE_DNS) == ns and uuid.NAMESPACE_DNS.version == 1
assert str(uuid.uuid5(uuid.NAMESPACE_DNS, "example.com")) == "cfbff0d1-9375-5685-968c-48ce8b15ae17"
assert uuid.uuid5(uuid.NAMESPACE_DNS, "x") == uuid.uuid5(uuid.UUID(ns), "x")
assert {uuid.NAMESPACE_DNS: 1}[uuid.UUID(ns)] == 1

# UUIDs order by value, expose .bytes/.urn, compare safely to other types
u_lo = uuid.UUID(ns)
u_hi = uuid.UUID("6ba7b811-9dad-11d1-80b4-00c04fd430c8")
assert u_lo < u_hi and u_lo <= u_lo and u_hi > u_lo and not (u_lo >= u_hi)
assert sorted([u_hi, u_lo]) == [u_lo, u_hi] and max(u_lo, u_hi) == u_hi
assert u_lo.urn == "urn:uuid:" + ns and len(u_lo.bytes) == 16
assert u_lo != "x" and u_lo != 5

class Other:
    pass
assert u_lo != Other() and not (u_lo == Other())
try:
    u_lo < 3
    assert False, "expected TypeError"
except TypeError:
    pass

# --- bool is an int in int(), float() and sum() ---
assert int(True) == 1 and int(False) == 0
assert float(True) == 1.0 and float(False) == 0.0
assert sum([True, False, True]) == 2
assert sum(x > 2 for x in [1, 3, 4]) == 2
assert sum([True, 1.5]) == 2.5 and sum([True], 10) == 11
assert int(True) + int("3") == 4
assert "%g %f %e" % (True, True, False) == "1 1.000000 0.000000e+00"

# --- %g / format 'g' default to 6 significant digits ---
assert "%g" % 3.14159265358979 == "3.14159"
assert "%g" % 1234567.0 == "1.23457e+06"
assert "%g %g %g" % (0.1, 1e-5, 100000.0) == "0.1 1e-05 100000"
assert "%.3g|%10g|%-6g|" % (3.14159, 2.5, 2.5) == "3.14|       2.5|2.5   |"
assert f"{3.14159265358979:g} {1234567.0:g} {1e21:g}" == "3.14159 1.23457e+06 1e+21"
assert "{:g}".format(0.1 + 0.2) == "0.3"
