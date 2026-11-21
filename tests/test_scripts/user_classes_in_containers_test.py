# User-defined classes inside containers and library types: equality,
# ordering and the protocols Python applies to them there.
import copy
import functools
import heapq
import itertools
from collections import deque, namedtuple
from dataclasses import dataclass, field
from datetime import date, timedelta
from decimal import Decimal
from enum import Enum, IntEnum, StrEnum


class V:
    def __init__(self, v):
        self.v = v

    def __eq__(self, other):
        return isinstance(other, V) and self.v == other.v

    def __repr__(self):
        return "V(%r)" % (self.v,)


class SubV(V):
    pass


# Container equality calls __eq__ on nested instances, inherited too.
assert [V(1)] == [V(1)]
assert (V(1),) in [(V(1),)]
assert {"a": V(1)} == {"a": V(1)}
assert [[V(1)], {"x": (V(1),)}] == [[V(1)], {"x": (V(1),)}]
assert [SubV(1)] == [SubV(1)] and SubV(1) in [SubV(1)]
assert deque([V(1)]) == deque([V(1)])
assert V(1) in deque([V(0), V(1)])
assert deque([V(0), V(1)]).index(V(1)) == 1
assert deque([V(1), V(1)]).count(V(1)) == 2
dq = deque([V(0), V(1)])
dq.remove(V(1))
assert list(dq) == [V(0)]
assert (V(1), V(1)).count(V(1)) == 2
assert [V(0), V(1)].index(V(1)) == 1


# == reflects: a builtin on the left asks the instance on the right.
class AnyEq:
    def __eq__(self, other):
        return other == 1


assert 1 == AnyEq() and not ("x" == AnyEq()) and not (1 != AnyEq())


class Truthy:
    def __eq__(self, other):
        return [1]  # a non-bool result counts by its truth


assert Truthy() == 0
assert not (V(1) == 1) and not (1 == V(1))


# Ordering: the first unequal elements decide, so equal but unorderable
# elements are passed over.
assert (V(1), 2) < (V(1), 3)
assert sorted([(V(1), 2), (V(1), 1)]) == [(V(1), 1), (V(1), 2)]
assert (None, 1) < (None, 2)


@functools.total_ordering
class T:
    def __init__(self, v):
        self.v = v

    def __eq__(self, other):
        return self.v == other.v

    def __lt__(self, other):
        return self.v < other.v

    def __hash__(self):
        return hash(self.v)


assert T(1) <= T(2) and T(3) >= T(2) and not (T(1) > T(2)) and T(2) > T(1)
assert [t.v for t in sorted([T(3), T(1), T(2)])] == [1, 2, 3]
h = [T(2), T(3), T(1)]
heapq.heapify(h)
assert heapq.heappop(h).v == 1


# Dataclasses: fields compare with ==, order compares like a tuple, and
# order is off by default.
@dataclass
class DC:
    v: object


@dataclass(order=True, frozen=True)
class DCO:
    v: object


assert DC(V(1)) == DC(V(1))
assert DC([V(1)]) == DC([V(1)])
assert sorted([DCO(Decimal("2.5")), DCO(Decimal("1.5"))]) == [DCO(Decimal("1.5")), DCO(Decimal("2.5"))]
assert DCO((1, "b")) < DCO((1, "c"))
assert DCO(date(2020, 1, 1)) < DCO(date(2020, 1, 2))
for bad in (lambda: DC(1) < DC(2), lambda: DCO(1) < DC(2)):
    try:
        bad()
        raise AssertionError("ordered values that have no order")
    except TypeError:
        pass


@dataclass
class Own:
    v: int

    def __repr__(self):
        return "own"

    def __eq__(self, other):
        return True


assert repr(Own(1)) == "own" and Own(1) == Own(2)


@dataclass
class WithList:
    v: int
    items: list = field(default_factory=list)


assert WithList(1).items is not WithList(1).items

# Named tuples are tuples.
NT = namedtuple("NT", "a b")
assert NT(V(1), 2) == NT(V(1), 2)
assert NT(V(1), V(1)).count(V(1)) == 2
assert NT(1.0, V(1)).index(V(1)) == 1
assert NT(1, 2) + (3,) == (1, 2, 3) and (0,) + NT(1, 2) == (0, 1, 2)
assert NT(1, 2) * 2 == (1, 2, 1, 2)
try:
    NT(1, 2).a = 5
    raise AssertionError("set a named tuple field")
except AttributeError:
    pass

# groupby compares keys with ==.
assert [(k, len(list(g))) for k, g in itertools.groupby([V(1), V(1), V(2)])] == [(V(1), 2), (V(2), 1)]
assert [(k, len(list(g))) for k, g in itertools.groupby([1, 1.0, True, 2])] == [(1, 3), (2, 1)]

# Immutable values are shared by copy; enum members stay singletons.


class Color(Enum):
    RED = 1


assert copy.deepcopy([Color.RED])[0] is Color.RED
assert copy.copy(Color.RED) is Color.RED
assert copy.deepcopy(Decimal("1.5")) == Decimal("1.5")
assert Color(1.0) is Color.RED
assert format(Color.RED, ">12") == "   Color.RED"


# IntEnum and StrEnum members act as their int and str values.
class Num(IntEnum):
    ONE = 1
    TWO = 2


class Txt(StrEnum):
    A = "ab"
    B = "cd"


assert Num.ONE + Num.TWO == 3 and 1 + Num.ONE == 2 and -Num.TWO == -2
assert Num.TWO / 4 == 0.5 and Num.TWO // 4 == 0 and [10, 20, 30][Num.ONE] == 20
assert list(range(Num.TWO)) == [0, 1] and isinstance(Num.ONE, int)
assert sum([Num.ONE, Num.TWO]) == 3
assert Txt.A.upper() == "AB" and len(Txt.A) == 2 and "a" in Txt.A
assert Txt.A + "z" == "abz" and "z" + Txt.A == "zab" and Txt.A < Txt.B
assert sorted([Txt.B, Txt.A]) == [Txt.A, Txt.B] and isinstance(Txt.A, str)
assert list(Txt.A) == ["a", "b"] and Txt.A[1] == "b"


# __index__, the sequence protocol, __delattr__, context managers.
class Idx:
    def __index__(self):
        return 1


assert [10, 20, 30][Idx()] == 20 and "xyz"[Idx()] == "y"
assert list(range(Idx())) == [0] and hex(Idx()) == "0x1"


class Seq:
    def __getitem__(self, i):
        if i >= 3:
            raise IndexError(i)
        return i * 10

    def __len__(self):
        return 3


assert list(reversed(Seq())) == [20, 10, 0]


class Attrs:
    def __init__(self):
        object.__setattr__(self, "store", {"v": 1})

    def __getattr__(self, name):
        if name in self.store:
            return self.store[name]
        raise AttributeError(name)

    def __delattr__(self, name):
        del self.store[name]


a = Attrs()
del a.v
assert not hasattr(a, "v")
a2 = Attrs()
delattr(a2, "v")
assert not hasattr(a2, "v")


class NotCM:
    pass


try:
    with NotCM():
        pass
    raise AssertionError("with accepted a non context manager")
except TypeError:
    pass

# Immutable library values refuse attribute writes.
for value in (date(2020, 1, 1), timedelta(1), Decimal("1")):
    try:
        value.extra = 1
        raise AssertionError("set an attribute on an immutable value")
    except AttributeError:
        pass


# super() finds the instance whatever the first parameter is called, and
# every instance has object's default dunders.
class Base:
    def __init__(s, v):
        s.v = v


class Child(Base):
    def __init__(s, v):
        super().__init__(v * 2)


assert Child(2).v == 4
p = Base(1)
assert p.__eq__(p) is True and p.__eq__(1) is NotImplemented
assert p.__hash__() == hash(p) and p.__repr__() == repr(p) and str(p) == p.__str__()

# Errors name the type and show reprs.
try:
    for x, y in [Base(1)]:
        pass
except TypeError as e:
    assert "Base" in str(e)
try:
    int(Base(1))
except TypeError as e:
    assert "'Base'" in str(e)
try:
    Color("x")
except ValueError as e:
    assert str(e) == "'x' is not a valid Color"

print("OK")
