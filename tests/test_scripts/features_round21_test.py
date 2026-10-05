# Round-21 features: functools.lru_cache/@cache, enum, dataclasses (+typing
# placeholders), and phase-1 generators. Every behavior diffed against
# CPython 3.14.

# --- functools.lru_cache -----------------------------------------------------
import functools

calls = []

@functools.lru_cache(maxsize=3)
def fib(n):
    calls.append(n)
    return n if n < 2 else fib(n - 1) + fib(n - 2)

assert fib(10) == 55
assert len(calls) == 11  # each value computed once
info = fib.cache_info()
assert info.hits > 0 and info.currsize <= 3 and info.misses == 11

@functools.lru_cache
def double(x):
    calls.append(("d", x))
    return x * 2

assert double(4) == 8 and double(4) == 8
assert ("d", 4) in calls and calls.count(("d", 4)) == 1

@functools.cache
def tri(x):
    return x * 3

assert tri(5) == 15 and tri(5) == 15

@functools.lru_cache(maxsize=2)
def bounded(x):
    return x * 10

bounded(1)
bounded(2)
bounded(3)  # evicts 1
bounded(1)  # recomputes
bi = bounded.cache_info()
assert bi.currsize == 2 and bi.misses == 4

bounded.cache_clear()
assert bounded.cache_info().currsize == 0

@functools.lru_cache
def kwfn(a, b=2):
    return a * b

assert kwfn(3) == 6 and kwfn(3, b=2) == 6  # same cache entry
assert kwfn(3, b=3) == 9

@functools.lru_cache
def unhashable_arg(x, lst):
    return len(lst)

try:
    unhashable_arg(1, [1, 2])
    assert False, "expected TypeError"
except TypeError as e:
    assert "unhashable" in str(e)

# --- enum --------------------------------------------------------------------
import enum

class Color(enum.Enum):
    RED = 1
    GREEN = 2
    BLUE = 3

assert Color.RED.name == "RED" and Color.RED.value == 1
assert repr(Color.RED) == "<Color.RED: 1>"
assert str(Color.RED) == "Color.RED"
assert Color.RED is Color.RED
assert Color.RED == Color.RED and Color.RED != Color.BLUE
assert Color(2) is Color.GREEN
try:
    Color(99)
    assert False, "expected ValueError"
except ValueError as e:
    assert "99 is not a valid Color" in str(e)
assert [m.name for m in Color] == ["RED", "GREEN", "BLUE"]
assert len(list(Color)) == 3
assert Color.BLUE in Color and Color.RED not in (Color.BLUE,)
assert isinstance(Color.RED, Color)
d = {Color.RED: "r"}
assert d[Color.RED] == "r"
assert Color.RED in [Color.RED, Color.GREEN]

class Traffic(enum.Enum):
    GO = "go"
    WAIT = "wait"
    STOP = "stop"

assert Traffic.GO.value == "go"
assert Traffic.WAIT.value == "wait"
assert Traffic.STOP.value == "stop"

# auto() continues from the previous integer value, as in Python.
class Continue(enum.Enum):
    FIVE = 5
    SIX = enum.auto()
    SEVEN = enum.auto()

assert Continue.SIX.value == 6 and Continue.SEVEN.value == 7

class Fresh(enum.Enum):
    A = enum.auto()
    B = enum.auto()

assert Fresh.A.value == 1 and Fresh.B.value == 2

try:
    class BadAuto(enum.Enum):
        S = "str"
        N = enum.auto()
    assert False, "expected TypeError"
except TypeError:
    pass

class Num(enum.IntEnum):
    ONE = 1
    TWO = 2

assert Num.ONE == 1 and Num.TWO != 1
assert Num.ONE + 10 == 11 and Num.TWO * 2 == 4
assert Num.ONE < 5 and Num.TWO >= 2
assert Num(2) is Num.TWO

class Aliased(enum.Enum):
    A = 1
    B = 1  # aliases to A

assert Aliased.B is Aliased.A and len(list(Aliased)) == 1

# --- dataclasses --------------------------------------------------------------
from dataclasses import dataclass, field, MISSING
from typing import Optional, List, Dict, Any

@dataclass
class Point:
    x: int
    y: int = 0

assert Point(1, 2).y == 2
assert Point(3).y == 0
assert repr(Point(1, 2)) == "Point(x=1, y=2)"
assert Point(1, 2) == Point(1, 2)
assert Point(1, 2) != Point(3)
kw = Point(y=9, x=8)
assert kw.x == 8 and kw.y == 9

@dataclass
class Config:
    name: str
    tags: List[str] = field(default_factory=list)
    ratio: float = 1.0

c1 = Config("main")
c1.tags.append("a")
assert c1.tags == ["a"] and Config("x").tags == []
assert repr(Config("n", ["t"], 0.5)) == "Config(name='n', tags=['t'], ratio=0.5)"

try:
    Point()
    assert False, "expected TypeError"
except TypeError as e:
    assert "missing 1 required positional argument: 'x'" in str(e)

@dataclass
class Typed:
    a: Optional[int] = None
    m: Dict[str, Any] = field(default_factory=dict)

t = Typed()
t.m["k"] = 1
assert t.a is None and t.m == {"k": 1} and Typed().m == {}

assert MISSING is MISSING

def annotated_fn(x: int, s: str = "d") -> str:
    return s + str(x)

assert annotated_fn(5) == "d5"
assert annotated_fn(1, "v") == "v1"

# --- generators (phase 1) ------------------------------------------------------
def count_up(n):
    i = 0
    while i < n:
        yield i
        i += 1

assert list(count_up(5)) == [0, 1, 2, 3, 4]
assert sum(count_up(10)) == 45

def squares(xs):
    for x in xs:
        v = x * x
        yield v
        yield v * 10

assert list(squares([1, 2])) == [1, 10, 4, 40]

def two_vals():
    yield "a"
    yield "b"

g = two_vals()
assert next(g) == "a" and next(g) == "b"
try:
    next(g)
    assert False, "expected StopIteration"
except StopIteration:
    pass

def evens(xs):
    for x in xs:
        if x % 2:
            continue
        yield x

assert list(evens(range(10))) == [0, 2, 4, 6, 8]

def until_big(xs):
    for x in xs:
        if x > 3:
            break
        yield x

assert list(until_big([1, 9, 2, 9, 3, 4, 5])) == [1]  # break exits at the first 9

# Side effects run exactly once, in order, around each yield.
log = []

def logging_gen():
    for x in [1, 2, 3]:
        log.append(("pre", x))
        yield x
        log.append(("post", x))

g2 = logging_gen()
assert next(g2) == 1
assert log == [("pre", 1)]
assert next(g2) == 2
assert log == [("pre", 1), ("post", 1), ("pre", 2)]

# Locals persist across yields; parameters bind at creation.
def accumulator(start):
    total = start
    for x in [10, 20]:
        total += x
        yield total

assert list(accumulator(5)) == [15, 35]

# Generators compose with the iteration protocol everywhere.
assert sorted(count_up(4), reverse=True) == [3, 2, 1, 0]
assert list(map(lambda v: v * 2, count_up(3))) == [0, 2, 4]
assert {v: v for v in "ab"} == {"a": "a", "b": "b"}

# Bare yields produce None.
def bare():
    yield

assert list(bare()) == [None]

# Nested no-yield loops inside a generator are fine.
def with_inner_loop(xs):
    for x in xs:
        total = 0
        for y in range(x):
            total += y
        yield total

assert list(with_inner_loop([3, 4])) == [3, 6]

print("features_round21_test passed")
