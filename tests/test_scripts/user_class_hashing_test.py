# Hashing of user-defined classes: equal instances are one key wherever a
# key is taken (dicts, sets, tuples used as keys, Counter, lru_cache, ...),
# unequal instances that share a hash stay apart, and Python's rules for
# when an instance is hashable at all.
import functools
import statistics
from collections import Counter, OrderedDict, defaultdict, namedtuple
from dataclasses import dataclass
from enum import IntEnum, StrEnum


class H:
    def __init__(self, k):
        self.k = k

    def __eq__(self, other):
        return isinstance(other, H) and self.k == other.k

    def __hash__(self):
        return hash(self.k)

    def __repr__(self):
        return "H(%r)" % (self.k,)


class SubH(H):
    pass


# Inside tuples and frozensets.
assert {(H(1), 1): "t"}[(H(1), 1)] == "t"
assert len({(H(1),), (H(1),)}) == 1
assert (H(1), 2) in {(H(1), 2)}
assert {((H(1),), "z"): 1}.get(((H(1),), "z")) == 1
assert {(H(1), "a", 2.0): 1}[(H(1), "a", 2)] == 1
assert hash((H(1), 1)) == hash((H(1), 1))
assert {frozenset([H(1)]): 1}[frozenset([H(1)])] == 1
assert frozenset([H(1)]) == frozenset([H(1)])

# Top level, and through an inherited __hash__.
assert {H(1): 1}[H(1)] == 1
assert len({H(1), H(1), H(2)}) == 2
assert {SubH(1): 1}[SubH(1)] == 1
assert {H(1): "base"}[SubH(1)] == "base"

# Set and dict operations.
assert len({H(1), H(2)} | {H(1)}) == 2
assert len({(H(1),)} & {(H(1),)}) == 1
assert len({H(1), H(2)} - {H(1)}) == 1
assert len(dict.fromkeys([H(1), H(1), H(2)])) == 2
s = {H(1), H(2)}
s.discard(H(1))
assert len(s) == 1
assert {H(1): 3}.pop(H(1)) == 3
d = {}
d.setdefault(H(1), []).append(1)
d.setdefault(H(1), []).append(2)
assert d == {H(1): [1, 2]}
assert len({H(1): 1}.keys() & {H(1): 2}.keys()) == 1

# Library containers.
assert Counter([H(1), H(1), H(2)]).most_common(1) == [(H(1), 2)]
assert Counter([(H(1),), (H(1),)]).most_common(1) == [((H(1),), 2)]
assert (Counter([H(1)]) + Counter([H(1)]))[H(1)] == 2
assert Counter([H(1)]) == Counter([H(1)])
c = Counter()
c[H(1)] = 5
assert c[H(1)] == 5 and H(1) in c
del c[H(1)]
assert len(c) == 0
assert OrderedDict([(H(1), 1)])[H(1)] == 1
dd = defaultdict(list)
dd[H(1)].append(1)
dd[H(1)].append(2)
assert len(dd) == 1
assert statistics.mode([H(1), H(2), H(2)]) == H(2)
assert statistics.multimode([H(1), H(1), H(2), H(2)]) == [H(1), H(2)]

calls = []


@functools.lru_cache(maxsize=None)
def cached(x):
    calls.append(x)
    return len(calls)


cached(H(1))
cached(H(1))
cached((H(2),))
cached((H(2),))
assert len(calls) == 2


# Unequal instances with the same hash stay apart.
class K:
    def __hash__(self):
        return 1


assert len({K(), K()}) == 2
assert len({(K(),), (K(),)}) == 2


class Collide:
    def __init__(self, v):
        self.v = v

    def __eq__(self, other):
        return isinstance(other, Collide) and self.v == other.v

    def __hash__(self):
        return 7


assert len({Collide(1), Collide(2), Collide(1)}) == 2
assert {Collide(1): "a", Collide(2): "b"}[Collide(2)] == "b"
assert Counter([Collide(1), Collide(2), Collide(1)])[Collide(1)] == 2


# A plain instance hashes by identity, everywhere.
class Plain:
    pass


p = Plain()
assert len(Counter([p, p, Plain()])) == 2
assert {(p, 1): 2}[(p, 1)] == 2
assert hash(p) == hash(p)


# Unhashable: __eq__ without __hash__ (also when overriding a base's
# __eq__), and __hash__ = None.
class EqOnly:
    def __eq__(self, other):
        return True


class Override(H):
    def __eq__(self, other):
        return False


class NoHash:
    __hash__ = None


for make in (EqOnly, lambda: Override(1), NoHash):
    for use in (lambda o: {o: 1}, lambda o: {o}, lambda o: hash(o), lambda o: {(o,): 1}, lambda o: Counter([o])):
        try:
            use(make())
            raise AssertionError("hashed an unhashable instance")
        except TypeError:
            pass
assert EqOnly.__hash__ is None and Override.__hash__ is None
try:
    cached(EqOnly())
    raise AssertionError("lru_cache took an unhashable argument")
except TypeError:
    pass


# Dataclasses follow Python's __hash__ table.
@dataclass
class DC:
    v: int


@dataclass(frozen=True)
class DCF:
    v: object


@dataclass(unsafe_hash=True)
class DCU:
    v: int


@dataclass(eq=False)
class DCNE:
    v: int


try:
    {DC(1): 1}
    raise AssertionError("an eq dataclass hashed")
except TypeError:
    pass
assert DC.__hash__ is None
assert {DCF(H(1)): 1}[DCF(H(1))] == 1
assert hash(DCF(H(1))) == hash(DCF(H(1)))
assert {DCU(1): 1}[DCU(1)] == 1
assert len({DCNE(1), DCNE(1)}) == 2

# A named tuple is a tuple: it keys as one.
NT = namedtuple("NT", "a b")
assert {(1, 2): "t"}[NT(1, 2)] == "t"
assert {NT(1, 2): "n"}[(1, 2)] == "n"
assert {NT(H(1), 2): 1}[NT(H(1), 2)] == 1
assert hash(NT(1, 2)) == hash((1, 2))


# IntEnum and StrEnum members key as their values.
class Num(IntEnum):
    ONE = 1


class Txt(StrEnum):
    A = "a"


assert {1: "one"}[Num.ONE] == "one"
assert {Num.ONE: "n"}[1] == "n"
assert {"a": 1}[Txt.A] == 1
assert hash(Num.ONE) == hash(1)
assert 1 == Num.ONE and 1.0 == Num.ONE

print("OK")
