# Round-22 review fixes: lazy itertools over generators (DoS), int overflow
# errors, order=True, display/call unpacking, raise-from, dict(defaultdict),
# namedtuple unpacking, leading-dot floats, wraps/vars/fsum/monotonic,
# catchable arity TypeErrors, clear generator limits, asdict/frozen.

import itertools

# --- itertools over an INFINITE generator terminates (was an OOM DoS) --------
def fib():
    a, b = 0, 1
    while True:
        yield a
        a, b = b, a + b

assert list(itertools.islice(fib(), 10)) == [0, 1, 1, 2, 3, 5, 8, 13, 21, 34]

count = 0
for v in fib():
    count += 1
    if v > 100:
        break
assert count == 13

def naturals():
    i = 0
    while True:
        yield i
        i += 1

assert list(itertools.takewhile(lambda x: x < 5, naturals())) == [0, 1, 2, 3, 4]
assert list(itertools.dropwhile(lambda x: x < 5, [4, 6, 3, 7])) == [6, 3, 7]
assert list(itertools.islice(itertools.dropwhile(lambda x: x < 5, naturals()), 3)) == [5, 6, 7]

c = itertools.chain([9, 9], fib())
assert (next(c), next(c), next(c)) == (9, 9, 0)

z = itertools.zip_longest(fib(), "ab")
assert next(z) == (0, "a") and next(z) == (1, "b")

cy = itertools.cycle(itertools.islice(fib(), 3))
assert [next(cy) for _ in range(7)] == [0, 1, 1, 0, 1, 1, 0]

assert list(itertools.compress(naturals(), [1, 0, 1])) == [0, 2]
assert list(itertools.accumulate(itertools.islice(fib(), 5))) == [0, 1, 2, 4, 7]

# --- int overflow raises catchable OverflowError -------------------------------
for expr in (lambda: 10**20, lambda: 2**64, lambda: 2**63,
             lambda: 9223372036854775807 + 1, lambda: (-9223372036854775807 - 1) - 1,
             lambda: (1 << 62) << 2, lambda: 3 * 3074457345618258603):
    try:
        expr()
        raised = False
    except OverflowError:
        raised = True
    assert raised, "expected OverflowError"

# In-range values still compute exactly.
assert 3037000500 * 3037000499 == 9223372033963249500
assert 1 << 62 == 4611686018427387904
assert -(2**62) * 2 == (-9223372036854775807 - 1)
assert 3 * 3074457345618258602 == 9223372036854775806
assert 2**10 == 1024

# --- dataclass(order=True) + unorderable instances ------------------------------
from dataclasses import dataclass, field, asdict, astuple

@dataclass(order=True)
class P:
    x: int
    y: int = 0

assert sorted([P(2), P(1), P(1, 5)]) == [P(1), P(1, 5), P(2)]
assert P(1) < P(2) and P(2) <= P(2) and P(3) > P(2) and not (P(2) >= P(3))

class Q:
    def __init__(self, v):
        self.v = v

try:
    sorted([Q(1), Q(2)])
    assert False, "expected TypeError"
except TypeError:
    pass

# --- display and call unpacking ---------------------------------------------------
d = {"x": 1, "y": 2}
assert {**d, "z": 3}["z"] == 3
assert {"z": 9, **d}["x"] == 1          # earlier entries can be overridden
assert {**d, "x": 100}["x"] == 100      # later entries win

a = [1, 2]
assert [*a, 3, *a] == [1, 2, 3, 1, 2]
assert [*"ab"] == ["a", "b"]
assert sorted({*{1, 2}, 3, *{1, 2}}) == [1, 2, 3]
assert {*"ab"} == {"a", "b"}

def kw(a, b=0, c=0):
    return (a, b, c)
assert kw(**{"a": 1}) == (1, 0, 0)
assert kw(1, **{"b": 2}) == (1, 2, 0)
assert kw(**{"a": 1, "c": 3}) == (1, 0, 3)

from collections import defaultdict
dd = defaultdict(list)
dd["a"].append(1)
dd["b"] = 2
assert dict(dd)["a"] == [1] and dict(dd)["b"] == 2 and len(dict(dd)) == 2
assert {**dd}["b"] == 2

# --- raise X from e ------------------------------------------------------------
try:
    1 / 0
except ZeroDivisionError as e:
    try:
        raise ValueError("wrapped") from e
    except ValueError as v:
        assert str(v) == "wrapped"

# --- namedtuple unpacking + leading-dot floats -----------------------------------
from collections import namedtuple
P2 = namedtuple("P2", ["x", "y"])
p = P2(3, 4)
x, y = p
assert (x, y) == (3, 4)
assert list(p) == [3, 4]
assert .5 + .25 == 0.75
assert [.5, 1.5] == [0.5, 1.5]

# --- namedtuples behave like tuples ---------------------------------------------------
P3 = namedtuple("P3", ["x", "y"])
q = P3(10, 20)
assert q[0] == 10 and q[1] == 20 and q[-1] == 20 and q[-2] == 10
assert len(q) == 2
assert list(q) == [10, 20] and tuple(q) == (10, 20)
a1, b1 = q
assert (a1, b1) == (10, 20)
try:
    q[5]
    assert False, "expected IndexError"
except IndexError:
    pass
try:
    q[-3]
    assert False, "expected IndexError"
except IndexError:
    pass
assert q["x"] == 10  # field lookup kept as an extension

# --- float literal forms ---------------------------------------------------------------
assert .5 + .25 == 0.75
assert 1. == 1.0 and 1.e2 == 100.0 and 1.5e2 == 150.0
assert 2.e-1 == 0.2
assert [1., .25] == [1.0, 0.25]
assert (5).bit_length() == 3  # parenthesised attribute access still works

# --- functools.wraps / vars / math.fsum / time.monotonic --------------------------
import functools
import math
import time

def deco(fn):
    @functools.wraps(fn)
    def wrapper(*args, **kwargs):
        return fn(*args, **kwargs)
    return wrapper

@deco
def named(a):
    return a * 2

assert named(3) == 6

assert math.fsum([0.1] * 10) == 1.0
assert abs(math.fsum([0.1, 0.2, 0.3]) - 0.6) < 1e-15
assert time.monotonic() >= 0

scope_var = 5
assert vars()["scope_var"] == 5

# --- arity errors are catchable TypeErrors ------------------------------------------
def two_args(a, b):
    return a

try:
    two_args(1)
    assert False, "expected TypeError"
except TypeError:
    pass

# --- clear limits for generator phase-1 gaps ------------------------------------------
def sgen():
    yield 1

gg = sgen()
try:
    gg.send(5)
    assert False, "expected NotImplementedError"
except NotImplementedError:
    pass

# --- asdict / astuple / frozen ----------------------------------------------------------
@dataclass
class Inner:
    v: int

@dataclass
class Outer:
    a: int
    inners: list = field(default_factory=list)

assert asdict(Outer(1, [Inner(2), Inner(3)])) == {"a": 1, "inners": [{"v": 2}, {"v": 3}]}
assert astuple(Inner(5)) == (5,)

@dataclass(frozen=True)
class F:
    x: int
    y: int = 0

f = F(1)
assert f == F(1)
assert {f: "k"}[F(1)] == "k"
try:
    f.x = 9
    assert False, "expected FrozenInstanceError"
except AttributeError:
    pass

print("review_round22_test passed")
