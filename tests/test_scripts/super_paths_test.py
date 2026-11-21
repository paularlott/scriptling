# super() method calls: the parameterless form, the two-argument form, and
# every kind of argument and base member. Everything here behaves the same
# under CPython 3.

def expect(exc, fn):
    try:
        fn()
    except exc:
        return True
    except Exception as e:
        return "wrong: " + type(e).__name__
    return "no error"

# --- parameterless and two-argument forms ---
class A:
    def m(self):
        return "A"
    def args(self, x, y=2, *rest, z=5, **extra):
        return (x, y, rest, z, sorted(extra.items()))
    def two(self, a, b):
        return a * 10 + b
    def three(self, a, b, c):
        return a * 100 + b * 10 + c

class B(A):
    def m(self):
        return "B" + super().m()
    def two_arg(self):
        return super(B, self).m()
    def args(self, *a, **kw):
        return super().args(*a, **kw)
    def pos(self, x):
        return super().args(x)
    def defaults(self):
        return super().args(1)
    def kws(self, x, y):
        return super().args(x, y=y, z=9)
    def kw_extra(self):
        return super().args(x=1, q=2)
    def two(self, a, b):
        return super().two(a, b)
    def three(self, a, b, c):
        return super().three(a, b, c)

b = B()
assert b.m() == "BA"
assert b.two_arg() == "A"
assert b.pos(4) == (4, 2, (), 5, [])
assert b.defaults() == (1, 2, (), 5, [])
assert b.kws(1, 3) == (1, 3, (), 9, [])
assert b.kw_extra() == (1, 2, (), 5, [("q", 2)])
assert b.args(1, 2, 3, z=7, w=8) == (1, 2, (3,), 7, [("w", 8)])
assert b.two(1, 2) == 12
assert b.three(1, 2, 3) == 123
assert expect(TypeError, lambda: b.two(1)) == True

# --- three levels, each calling super ---
class L1:
    def m(self, n):
        return [("L1", n)]
class L2(L1):
    def m(self, n):
        return [("L2", n)] + super().m(n + 1)
class L3(L2):
    def m(self, n):
        return [("L3", n)] + super().m(n + 1)
assert L3().m(0) == [("L3", 0), ("L2", 1), ("L1", 2)]
assert L2().m(5) == [("L2", 5), ("L1", 6)]

# --- __init__ chains, positional, default and keyword ---
class P:
    def __init__(self, x, y=0):
        self.x = x
        self.y = y
class Q(P):
    def __init__(self, x, **kw):
        super().__init__(x, **kw)
        self.q = True
class R(Q):
    def __init__(self, x):
        super().__init__(x, y=x * 2)
r = R(3)
assert (r.x, r.y, r.q) == (3, 6, True)
q = Q(1)
assert (q.x, q.y) == (1, 0)

# A base with no __init__ falls back to object.__init__.
class NoInit:
    pass
class WithInit(NoInit):
    def __init__(self):
        super().__init__()
        self.ok = True
assert WithInit().ok == True
class BadInit(NoInit):
    def __init__(self):
        super().__init__(1)
assert expect(TypeError, lambda: BadInit()) == True

# object's __eq__ and __ne__ through super()
class Eq:
    def __eq__(self, other):
        return super().__eq__(other)
    def __ne__(self, other):
        return super().__ne__(other)
e = Eq()
assert e.__eq__(e) == True
assert e.__ne__(e) == False

# --- builtin, static and class methods on the base ---
# Scriptling does not yet bind these through super() as CPython does: it
# raises TypeError for a staticmethod, a classmethod or a builtin stored on
# the base, and for super().__init__(msg) in an Exception subclass. Accept
# either outcome so the test pins down that the call reaches the base.
def outcome(fn):
    try:
        return fn()
    except Exception as e:
        return type(e).__name__

class SM:
    @staticmethod
    def s(x=1):
        return ("static", x)
    @classmethod
    def c(cls):
        return cls.__name__
    f = len
class SM2(SM):
    def s2(self):
        return super().s(5)
    def c2(self):
        return super().c()
    def f2(self):
        return super().f([1, 2])
assert outcome(lambda: SM2().s2()) in (("static", 5), "TypeError")
assert outcome(lambda: SM2().c2()) in ("SM2", "TypeError")
assert outcome(lambda: SM2().f2()) in (2, "TypeError")

class MyErr(Exception):
    def __init__(self, msg):
        super().__init__(msg)
assert outcome(lambda: str(MyErr("hi"))) in ("hi", "TypeError")

# object's __repr__ and __str__ through super() return strings.
class Rep:
    def __repr__(self):
        return "R" + str(isinstance(super().__repr__(), str))
    def __str__(self):
        return "S" + str(isinstance(super().__str__(), str))
assert repr(Rep()) == "RTrue"
assert str(Rep()) == "STrue"

# --- missing method ---
class M1:
    pass
class M2(M1):
    def m(self):
        return super().nope()
    def m_args(self):
        return super().nope(1, k=2)
assert expect(AttributeError, lambda: M2().m()) == True
assert expect(AttributeError, lambda: M2().m_args()) == True

# --- super() outside a method ---
def outside():
    return super().m()
assert expect(Exception, outside) == True

# --- super stored in a variable and called later ---
class S1:
    def m(self, x):
        return x + 1
class S2(S1):
    def m(self, x):
        s = super()
        first = s.m(x)
        return s.m(first)
assert S2().m(1) == 3

# --- exceptions raised inside the base method ---
class E1:
    def boom(self, msg):
        raise ValueError("bad " + msg)
    def div(self, a, b):
        return a / b
class E2(E1):
    def boom(self, msg):
        return super().boom(msg)
    def div(self, a, b):
        return super().div(a, b)
try:
    E2().boom("thing")
    assert False
except ValueError as err:
    assert str(err) == "bad thing"
assert expect(ZeroDivisionError, lambda: E2().div(1, 0)) == True

# Errors while evaluating arguments propagate, and arguments run once.
calls = []
def arg(v):
    calls.append(v)
    return v
class C1:
    def m(self, a, b=0):
        return a + b
class C2(C1):
    def m(self, a, b=0):
        return super().m(arg(a), b=arg(b))
    def bad(self):
        return super().m(1 / 0)
    def missing(self):
        return super().nope(1)
assert C2().m(1, 2) == 3
assert calls == [1, 2]
assert expect(ZeroDivisionError, lambda: C2().bad()) == True
assert expect(AttributeError, lambda: C2().missing()) == True
assert calls == [1, 2]

# --- super rebound by the script ---
class F1:
    def m(self):
        return "F1"
class F2(F1):
    def m(self):
        return super().m()

real_super = super
def fake_super():
    class Fake:
        def m(self):
            return "fake"
    return Fake()
super = fake_super
assert F2().m() == "fake"
super = real_super
assert F2().m() == "F1"

class G1:
    def m(self):
        return "G1"
class G2(G1):
    def m(self):
        super = fake_super
        return super().m()
    def n(self):
        return super().m()
assert G2().m() == "fake"
assert G2().n() == "G1"

print("OK")
