# Parameter defaults are evaluated once at definition time, in the defining
# scope, and bind that value — CPython semantics. Previously scriptling
# re-evaluated the default expression at call time against the (possibly
# mutated) defining scope.

# --- Late-binding workaround idiom now works -------------------------------
fns = []
for n in [11, 12, 13]:
    fns.append(lambda x, n=n: x + n)
assert [f(0) for f in fns] == [11, 12, 13]

fns = []
for n in [11, 12, 13]:
    def g(x, n=n):
        return x + n
    fns.append(g)
assert [f(0) for f in fns] == [11, 12, 13]

# Post-definition mutation of the default's source has no effect
n = 10
def f(x=n):
    return x
n = 99
assert f() == 10

# --- Default expression runs exactly once per definition --------------------
calls = []
def make():
    calls.append(1)
    return len(calls)

def once(x=make()):
    return x

assert once() == 1
assert once() == 1
assert once(5) == 5
assert len(calls) == 1

# Each loop iteration re-executes the def, so the default re-resolves
vals = []
for i in range(3):
    def h(x=i * 10):
        return x
    vals.append(h())
assert vals == [0, 10, 20]

# --- Mutable defaults are shared across calls (the classic CPython gotcha) --
def grow(item, acc=[]):
    acc.append(item)
    return acc

first = grow(1)
second = grow(2)
assert first == [1, 2]
assert second == [1, 2]

def with_dict(d={"count": 0}):
    d["count"] = d["count"] + 1
    return d["count"]

assert with_dict() == 1
assert with_dict() == 2

# --- Errors in defaults surface at definition -------------------------------
try:
    def bad(x=undefined_name):
        return x
    assert False, "expected NameError at def"
except Exception:
    pass

try:
    def zero(x=1 // 0):
        return x
    assert False, "expected ZeroDivisionError at def"
except ZeroDivisionError:
    pass

# After a failed def, the name is not bound
try:
    zero
    bound = True
except Exception:
    bound = False
assert not bound

# --- Interaction with keyword-only, *args, **kwargs -------------------------
def mixed(a, b=2, *, c=3):
    return (a, b, c)

assert mixed(1) == (1, 2, 3)
assert mixed(1, 9) == (1, 9, 3)
assert mixed(1, c=10) == (1, 2, 10)
assert mixed(1, b=8, c=9) == (1, 8, 9)

def varparams(a, b=2, *rest, **kw):
    return (a, b, list(rest), kw)

assert varparams(1) == (1, 2, [], {})
assert varparams(1, 5, 6, 7, k=8) == (1, 5, [6, 7], {"k": 8})

lam = lambda a, b=4, *rest: (a, b, list(rest))
assert lam(1) == (1, 4, [])
assert lam(1, 2, 3) == (1, 2, [3])

# Keyword fill of a defaulted parameter uses the resolved value
lam2 = lambda a, scale=10: a * scale
assert lam2(a=3) == 30

# --- Defaults in methods and decorated functions ----------------------------
scale = 2
class Calc:
    def __init__(self, factor=scale):
        self.factor = factor

    def apply(self, v, offset=100):
        return v * self.factor + offset

scale = 1000  # class body already ran; defaults keep their captured values
c = Calc()
assert c.apply(3) == 106
assert Calc(10).apply(3) == 130

deco_calls = []
def deco(fn):
    deco_calls.append(1)
    return fn

cell = {"n": 0}
def bump():
    cell["n"] = cell["n"] + 1
    return cell["n"]

@deco
def decorated(x=bump()):
    return x

assert decorated() == 1  # default resolved before the decorator ran
assert cell["n"] == 1
assert len(deco_calls) == 1

# --- Defaults referencing earlier definitions in the same scope -------------
base = 5
def derived(x=base + 1):
    return x
assert derived() == 6

print("defaults_capture_test passed")
