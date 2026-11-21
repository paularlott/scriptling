# Generator expressions are lazy generators, as in Python 3: they yield one
# value per next(), are consumed once, stop where a consumer stops, evaluate
# their first iterable when created and everything else as they run, and keep
# their loop variables to themselves.
import itertools

xs = [1, 2, 3, 4]

# --- a generator, not a list ---------------------------------------------
g = (x for x in xs)
assert type(g).__name__ == "generator"
assert iter(g) is g
assert (next(g), next(g), list(g)) == (1, 2, [3, 4])
assert list(g) == []
try:
    next(g)
    assert False, "exhausted generator expression must raise StopIteration"
except StopIteration:
    pass

g = (x for x in xs)
for op in (lambda: len(g), lambda: g[0]):
    try:
        op()
        assert False, "len() and indexing are TypeErrors on a generator"
    except TypeError:
        pass

# consumed once
g = (x * 10 for x in xs)
assert list(g) == [10, 20, 30, 40]
assert list(g) == []
assert sum(g) == 0

# a generator is truthy, even an empty one
assert bool(x for x in [])

# --- laziness ---------------------------------------------------------------
log = []


def chk(v):
    log.append(v)
    return v > 1


g = (chk(x) for x in xs)
assert log == []
assert next(g) is False
assert log == [1]

# short-circuiting consumers stop at the deciding element
log = []
assert any(chk(x) for x in xs)
assert log == [1, 2]
log = []
assert not all(not chk(x) for x in xs)
assert log == [1, 2]
log = []
assert next((x for x in xs if chk(x)), None) == 2
assert log == [1, 2]
log = []
assert next((x for x in xs if chk(x) and x > 9), "none") == "none"
assert log == [1, 2, 3, 4]

# `in` consumes up to the match
g = (x for x in xs)
assert 2 in g
assert list(g) == [3, 4]
g = (x for x in xs)
assert 9 not in g
assert list(g) == []

# infinite sources
assert list(itertools.islice((x * x for x in itertools.count()), 3)) == [0, 1, 4]
assert next(x for x in itertools.count(5) if x % 7 == 0) == 7
assert list(zip("ab", (x for x in itertools.count()))) == [("a", 0), ("b", 1)]
assert list(itertools.takewhile(lambda v: v < 3, (x for x in itertools.count()))) == [0, 1, 2]

# a large one is not built as a list first
assert sum(x for x in range(200000)) == 19999900000
assert max(x % 1000 for x in range(100000)) == 999

# interleaving: map and filter pull one element at a time
log = []
m = map(lambda v: v * 2, (log.append(x) or x for x in xs))
assert log == []
assert next(m) == 2
assert log == [1]
log = []
f = filter(lambda v: v > 2, (log.append(x) or x for x in xs))
assert next(f) == 3
assert log == [1, 2, 3]

# --- the first iterable is evaluated at creation, the rest lazily ----------
try:
    (x for x in undefined_name_1)
    assert False, "NameError expected at creation"
except NameError:
    pass

try:
    (x for x in 5)
    assert False, "TypeError expected at creation"
except TypeError:
    pass

# a later iterable, a condition or the element are not evaluated at creation
g = (y for x in xs for y in undefined_name_2)
g2 = (undefined_name_3 for x in xs)
g3 = (x for x in xs if undefined_name_4)
for gen in (g, g2, g3):
    try:
        next(gen)
        assert False, "NameError expected on first next()"
    except NameError:
        pass

# the first iterable is bound once: rebinding the name later changes nothing
src = [1, 2]
g = (x for x in src)
src = [7, 8, 9]
assert list(g) == [1, 2]

# but mutating the object it refers to is seen, as in Python
src = [1, 2]
g = (x for x in src)
src.append(3)
assert list(g) == [1, 2, 3]

# --- scoping and late binding ----------------------------------------------
x = "outer"
assert list(x for x in [1, 2]) == [1, 2]
assert x == "outer"

k = 10
g = (v + k for v in [1, 2])
k = 20
assert list(g) == [21, 22]

def adders(n):
    return (v * n for v in range(3))

a, b = adders(2), adders(3)
assert list(b) == [0, 3, 6]
assert list(a) == [0, 2, 4]


def counter():
    total = 0
    g = (v + total for v in range(3))
    total = 100
    return list(g)


assert counter() == [100, 101, 102]


# a generator expression returned from a function outlives the call that
# made it, and calls in between do not disturb its scope
def make(n):
    base = n * 100
    return (base + v for v in range(2))

gens = [make(i) for i in range(3)]
assert [list(gg) for gg in gens] == [[0, 1], [100, 101], [200, 201]]

# --- every enclosing context -------------------------------------------------
# lambda
sq = lambda seq: (v * v for v in seq)
assert list(sq([1, 2, 3])) == [1, 4, 9]
assert (lambda seq: sum(v for v in seq))([1, 2, 3]) == 6


# method
class Bag:
    def __init__(self, items):
        self.items = items

    def evens(self):
        return (v for v in self.items if v % 2 == 0)

    def total(self):
        return sum(v for v in self.items)


bag = Bag([1, 2, 3, 4])
assert list(bag.evens()) == [2, 4]
assert bag.total() == 10


# class body: the first iterable may use class-level names
class Table:
    rows = [1, 2, 3]
    doubled = list(r * 2 for r in rows)
    total = sum(r for r in rows)


assert Table.doubled == [2, 4, 6]
assert Table.total == 6

# inside list, set and dict comprehensions, and other generator expressions
assert [sum(v for v in range(n)) for n in range(4)] == [0, 0, 1, 3]
assert {n: list(v for v in range(n)) for n in range(3)} == {0: [], 1: [0], 2: [0, 1]}
assert sorted({sum(v for v in range(n)) for n in range(4)}) == [0, 1, 3]
assert list(list(v for v in range(n)) for n in range(3)) == [[], [0], [0, 1]]
assert list(y * 2 for y in (x + 1 for x in range(3))) == [2, 4, 6]
inner = (x for x in range(3))
outer = (y for y in inner if y)
assert list(outer) == [1, 2]
assert list(inner) == []


# inside a generator function: its own yield does not make the enclosing
# function a generator, and it does not upset the enclosing generator
def plain():
    return sum(v for v in range(4))


assert plain() == 6


def gen_fn():
    yield sum(v for v in range(3))
    g = (v * 2 for v in range(3))
    yield next(g)
    for v in (w + 1 for w in range(2)):
        yield v
    yield list(g)


assert list(gen_fn()) == [3, 0, 1, 2, [2, 4]]

# --- forms -------------------------------------------------------------------
d = {"a": 1, "b": 2}
assert list(k + str(v) for k, v in d.items()) == ["a1", "b2"]
assert list(k for (k, v) in d.items() if v > 1) == ["b"]
assert list(x for x in range(10) if x % 2 if x % 3) == [1, 5, 7]
assert list((x, y) for x in range(3) if x for y in "ab" if y != "a") == [(1, "b"), (2, "b")]
assert list(x * y for x in range(1, 3) for y in range(1, 3)) == [1, 2, 2, 4]
assert list((x if x % 2 else -x) for x in range(4)) == [0, 1, -2, 3]
assert list([v for v in range(n)] for n in range(3)) == [[], [0], [0, 1]]
assert list((n := v) for v in range(3)) == [0, 1, 2]
assert list(c.upper() for c in "abc") == ["A", "B", "C"]
assert list(v for v in (1, 2)) == [1, 2]
assert sorted(v for v in {3, 1, 2}) == [1, 2, 3]
assert list(k for k in {"x": 1, "y": 2}) == ["x", "y"]

# --- generator protocol: send and close ---------------------------------------
g = (x for x in range(5))
try:
    g.send(1)
    assert False, "send(non-None) to a just-started generator is a TypeError"
except TypeError:
    pass
g = (x for x in range(5))
assert g.send(None) == 0
assert g.send("ignored") == 1
g.close()
assert list(g) == []
g = (x for x in range(3))
g.close()
try:
    next(g)
    assert False
except StopIteration:
    pass

# --- an early break from a loop over one, then the rest -------------------------
g = (v * v for v in range(10))
for s in g:
    if s > 10:
        break
assert s == 16
assert next(g) == 25

# --- an exception midway ---------------------------------------------------------
def boom(v):
    if v == 3:
        raise ValueError("three")
    return v


g = (boom(v) for v in range(6))
got = []
try:
    for v in g:
        got.append(v)
    assert False, "ValueError expected"
except ValueError as e:
    assert str(e) == "three"
assert got == [0, 1, 2]
assert list(g) == []

g = (boom(v) for v in range(6))
try:
    list(g)
    assert False
except ValueError:
    pass
assert list(g) == []

# --- through the consumers --------------------------------------------------------
assert list(zip((x for x in "abc"), (y for y in range(10)))) == [("a", 0), ("b", 1), ("c", 2)]
assert list(enumerate((x * 2 for x in range(3)), 1)) == [(1, 0), (2, 2), (3, 4)]
assert list(map(str, (x for x in range(3)))) == ["0", "1", "2"]
assert list(filter(None, (x % 3 for x in range(6)))) == [1, 2, 1, 2]
assert list(itertools.chain((x for x in "ab"), (y for y in "cd"))) == ["a", "b", "c", "d"]
assert list(itertools.islice((x for x in range(100)), 2, 5)) == [2, 3, 4]
assert sorted((w for w in ["ccc", "a", "bb"]), key=len) == ["a", "bb", "ccc"]
assert sorted((x for x in range(4)), reverse=True) == [3, 2, 1, 0]
assert "".join(c * 2 for c in "ab") == "aabb"
assert ", ".join(str(x) for x in range(3)) == "0, 1, 2"
assert dict((k, len(k)) for k in ["a", "bb"]) == {"a": 1, "bb": 2}
assert set(x % 3 for x in range(9)) == {0, 1, 2}
assert frozenset(x for x in "aab") == frozenset("ab")
assert tuple(x for x in range(3)) == (0, 1, 2)
assert min((x for x in []), default=-1) == -1
assert max((x for x in []), default=-1) == -1
assert min((x for x in [3, 1, 2]), default=-1) == 1
assert max((len(s) for s in ["a", "abc"]), default=0) == 3
assert sum((x for x in range(4)), 10) == 16
assert sum(x > 1 for x in range(4)) == 2
assert [*(x for x in range(3)), 9] == [0, 1, 2, 9]
first, *rest = (x for x in range(4))
assert (first, rest) == (0, [1, 2, 3])
p, q = (x for x in "pq")
assert (p, q) == ("p", "q")


def star(*args):
    return args


assert star(*(x for x in range(3))) == (0, 1, 2)
lst = [9]
lst += (x for x in range(2))
assert lst == [9, 0, 1]
lst.extend(x for x in range(2))
assert lst == [9, 0, 1, 0, 1]
dd = {}
dd.update((k, 1) for k in "ab")
assert dd == {"a": 1, "b": 1}
assert bytes(x + 65 for x in range(3)) == b"ABC"

print("OK")
