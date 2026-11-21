# Calls, unpacking and string iteration: the shapes with dedicated fast paths.
# Everything here behaves the same under CPython 3.

# --- zero-argument functions (pooled call frames with no slots) ---
G = 5
def zero():
    return G
def nested():
    return zero() + zero()
assert zero() == 5
assert nested() == 10

def make_counter():
    n = [0]
    def bump():
        n[0] += 1
        return n[0]
    return bump
bump = make_counter()
assert [bump(), bump(), bump()] == [1, 2, 3]

def depth(n=[0]):
    return n
def rec():
    global G
    G -= 1
    if G > 0:
        return rec()
    return "done"
assert rec() == "done" and G == 0

# --- lambdas that return lambdas keep what they captured ---
v = 99
def outer_l(v):
    return lambda: (lambda: v)
h1 = outer_l(1)()
h2 = outer_l(2)()
assert (h1(), h2()) == (1, 2)
wrap = lambda: h1()
assert wrap() == 1
mk = lambda a: (lambda: a)
m1 = mk(1)
m2 = mk(2)
assert (m1(), m2()) == (1, 2)
mk2 = lambda a, b=10: (lambda c: a + b + c)
add1 = mk2(1)
add2 = mk2(2, b=20)
assert (add1(100), add2(100)) == (111, 122)
pick = lambda: [lambda: v][0]
assert pick()() == 99
deflam = lambda f=(lambda: 7): f()
assert deflam() == 7
assert sorted([3, 1, 2], key=lambda n: -n) == [3, 2, 1]

# --- keyword arguments to user functions ---
def f(a, b=10, c=20):
    return (a, b, c)
assert f(1) == (1, 10, 20)
assert f(1, c=3) == (1, 10, 3)
assert f(1, b=2) == (1, 2, 20)
assert f(a=1, c=3, b=2) == (1, 2, 3)
assert f(c=3, a=1) == (1, 10, 3)
assert f(1, 2, c=3) == (1, 2, 3)

def expect(exc, fn):
    try:
        fn()
    except exc:
        return True
    except Exception as e:
        return "wrong: " + type(e).__name__
    return "no error"
assert expect(TypeError, lambda: f(1, a=2)) == True
assert expect(TypeError, lambda: f(1, z=2)) == True
assert expect(TypeError, lambda: f(c=1)) == True
assert expect(TypeError, lambda: f(1, 2, 3, 4)) == True

# keyword values that call back through the same call site
def g(n, acc=0):
    if n <= 0:
        return acc
    return g(n - 1, acc=g(n - 2, acc=acc + 1))
assert g(6) == 20

# keyword-only, positional-only, *args and **kwargs
def ko(a, *, key, other=1):
    return (a, key, other)
assert ko(1, key=2) == (1, 2, 1)
assert ko(1, other=5, key=2) == (1, 2, 5)
assert expect(TypeError, lambda: ko(1)) == True
assert expect(TypeError, lambda: ko(1, 2)) == True

def po(a, b, /, c=3):
    return (a, b, c)
assert po(1, 2, c=4) == (1, 2, 4)
assert expect(TypeError, lambda: po(1, b=2)) == True

def va(a, *rest, flag=False, **extra):
    return (a, rest, flag, sorted(extra.items()))
assert va(1, 2, 3, flag=True, z=1, y=2) == (1, (2, 3), True, [("y", 2), ("z", 1)])
assert va(1) == (1, (), False, [])
assert va(1, **{"flag": True, "q": 9}) == (1, (), True, [("q", 9)])
assert va(1, x=1, **{"q": 9}) == (1, (), False, [("q", 9), ("x", 1)])

# generator functions, lambdas, classes, bound methods and builtins with keywords
def gen(start, step=1, count=3):
    for i in range(count):
        yield start + i * step
assert list(gen(1, count=2)) == [1, 2]
assert list(gen(1, step=10)) == [1, 11, 21]

lam = lambda x, y=2: x * y
assert lam(3, y=4) == 12

class P:
    def __init__(self, x, y=0):
        self.x = x
        self.y = y
    def moved(self, dx=0, dy=0):
        return P(self.x + dx, y=self.y + dy)
p = P(1, y=2).moved(dy=5)
assert (p.x, p.y) == (1, 7)
assert sorted([3, 1, 2], reverse=True) == [3, 2, 1]
assert "a,b,c".split(",", maxsplit=1) == ["a", "b,c"]

# defaults are bound per call and captured correctly by closures
def outer(a, b=2):
    def inner():
        return a + b
    return inner
assert outer(1)() == 3
assert outer(1, b=5)() == 6
assert outer(b=1, a=1)() == 2

def many(a, b=1, c=2, d=3, e=4, f=5, g=6, h=7, i=8, j=9):
    return a + b + c + d + e + f + g + h + i + j
assert many(0) == 45
assert many(0, j=0, b=0) == 35

# --- flat unpacking ---
a, b = 1, 2
a, b = b, a
assert (a, b) == (2, 1)
x, y, z = a + b, a * b, a - b
assert (x, y, z) == (3, 2, 1)
fa, fb = 0, 1
for _ in range(10):
    fa, fb = fb, fa + fb
assert fa == 55
w1, w2, w3, w4 = 1, 2, 3, 4
assert w1 + w2 + w3 + w4 == 10
v1, v2, v3, v4, v5 = 1, 2, 3, 4, 5
assert v5 == 5

# a failure while evaluating the values leaves every target untouched
m, n = "old-m", "old-n"
try:
    m, n = "new-m", {}["missing"]
except KeyError:
    pass
assert (m, n) == ("old-m", "old-n")

def unpack_locals():
    p, q = 1, 2
    p, q = q, p
    return p, q
assert unpack_locals() == (2, 1)

# starred unpacking gives a new list, not a view of the source
lst = [1, 2, 3]
first, *mid, last = lst
mid.append(9)
assert lst == [1, 2, 3]
assert mid == [2, 9]
head, *tail = lst
tail.append(4)
assert lst == [1, 2, 3]
*init, end = lst
init.append(0)
assert lst == [1, 2, 3]

# --- one-character strings ---
chars = []
for ch in "héllo wörld":
    chars.append(ch)
assert "".join(chars) == "héllo wörld"
assert len(chars) == 11
s = "abc"
assert s[0] == "a" and s[-1] == "c"
assert s[0] + s[0] == "aa"
assert list("ab") == ["a", "b"]
c0, c1 = "xy"
assert (c0, c1) == ("x", "y")
assert [c.upper() for c in "ab"] == ["A", "B"]
d = {}
for ch in "aab":
    d[ch] = d.get(ch, 0) + 1
assert d == {"a": 2, "b": 1}
assert "é"[0] == "é"
assert expect(IndexError, lambda: s[5]) == True

# --- split and rsplit take sep= and maxsplit= by keyword ---
assert "a,b,c".split(",", maxsplit=1) == ["a", "b,c"]
assert "a b c".split(maxsplit=1) == ["a", "b c"]
assert "a,b,c".rsplit(",", maxsplit=1) == ["a,b", "c"]
assert "a,b,c".split(sep=",") == ["a", "b", "c"]
assert b"a,b,c".split(b",", maxsplit=1) == [b"a", b"b,c"]
assert expect(TypeError, lambda: "a".split(",", sep=",")) == True
assert expect(TypeError, lambda: "a".split(foo=1)) == True

print("OK")
