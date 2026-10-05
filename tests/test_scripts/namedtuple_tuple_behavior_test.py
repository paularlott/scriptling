# namedtuples behave like tuples: indexing, slicing, iteration, ordering,
# equality, immutability, keyword construction, defaults.
from collections import namedtuple

P = namedtuple("P", "x y")
p = P(1, 2)

# Indexing, slicing, length, iteration, unpacking
assert p[0] == 1 and p[-1] == 2 and len(p) == 2
assert p[0:1] == (1,) and p[::-1] == (2, 1) and p[1:] == (2,) and p[:] == (1, 2)
assert list(p) == [1, 2] and tuple(p) == (1, 2)
x, y = p
assert (x, y) == (1, 2)
try:
    p[5]
    assert False, "expected IndexError"
except IndexError:
    pass

# Aggregates and membership
assert max(p) == 2 and sum(p) == 3 and 2 in p and 5 not in p

# Construction: positional, keyword, mixed, defaults, errors
assert P(x=1, y=2) == P(1, 2) == P(1, y=2) == P(y=2, x=1)
Q = namedtuple("Q", "a b c", defaults=(0, 9))
assert Q(1) == (1, 0, 9) and Q(1, 2) == (1, 2, 9) and Q(1, 2, 3) == (1, 2, 3)
for bad in (lambda: P(1), lambda: P(1, 2, 3), lambda: P(1, x=2), lambda: P(1, z=2)):
    try:
        bad()
        assert False, "expected TypeError"
    except TypeError:
        pass

# Equality, hashing, ordering
assert p == (1, 2) and p != (1, 3) and p != (1, 2, 3) and p != 5
assert {p: "v"}[P(1, 2)] == "v" and len({p, P(1, 2)}) == 1
assert sorted([P(2, 1), P(1, 5), P(1, 2)]) == [P(1, 2), P(1, 5), P(2, 1)]
assert P(1, 2) < P(1, 3) and P(1, 2) <= P(1, 2) and P(2, 0) > P(1, 9)
assert max([P(1, 2), P(3, 0)]) == P(3, 0) and min(P(1, 2), P(0, 9)) == P(0, 9)
R = namedtuple("R", ["a", "b"])
assert R("a", 1) < R("a", 1.5) < R("b", 0)
try:
    P(1, "a") < P(1, 2)
    assert False, "expected TypeError"
except TypeError:
    pass

# Immutability and metadata
try:
    p.x = 3
    assert False, "expected AttributeError"
except AttributeError:
    pass
assert p.x == 1
assert P._fields == ("x", "y") and p._fields == ("x", "y")
assert p._replace(x=9) == (9, 2) and p == (1, 2)
assert p._asdict() == {"x": 1, "y": 2}
assert repr(p) == "P(x=1, y=2)"

# Splat and extend accept namedtuples (and any __iter__ class)
def add(a, b):
    return a + b
assert add(*p) == 3
lst = [0]
lst.extend(p)
assert lst == [0, 1, 2]


class Bag:
    def __iter__(self):
        return iter([3, 1, 2])


b = Bag()
assert 1 in b and 9 not in b  # `in` falls back to __iter__
assert list(b) == [3, 1, 2]
lst2 = []
lst2.extend(b)
assert lst2 == [3, 1, 2]


class Plain:
    pass


try:
    1 in Plain()
    assert False, "expected TypeError"
except TypeError:
    pass
try:
    add(*Plain())
    assert False, "expected TypeError"
except TypeError:
    pass
