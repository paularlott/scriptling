# Library types show the values they hold as repr() does: an instance's own
# __repr__ (or str() falling back to it), nested at any depth, and cycles
# as Python prints them.
import itertools
from collections import Counter, OrderedDict, defaultdict, deque, namedtuple
from dataclasses import dataclass, field
from datetime import date
from decimal import Decimal
from enum import Enum, IntEnum


class R:
    def __init__(self, v):
        self.v = v

    def __repr__(self):
        return "R<%s>" % (self.v,)


class S:
    def __str__(self):
        return "S!"


class B:
    def __repr__(self):
        return "B-repr"

    def __str__(self):
        return "B-str"


NT = namedtuple("NT", "a b")


@dataclass
class DC:
    v: object
    items: list = field(default_factory=list)


@dataclass(frozen=True)
class Money:
    amount: Decimal
    day: date


class Color(Enum):
    RED = 1


class Num(IntEnum):
    TWO = 2


# deque, Counter, OrderedDict, defaultdict
assert repr(deque([R(1), [R(2)]])) == "deque([R<1>, [R<2>]])"
assert repr(deque([R(1)], maxlen=3)) == "deque([R<1>], maxlen=3)"
assert str(deque([B()])) == "deque([B-repr])"
assert repr(Counter({R(1): 2})) == "Counter({R<1>: 2})"
assert str(Counter([R(1), R(1)]).most_common()[0][0]) == "R<1>"
assert repr(OrderedDict([("k", R(1))])) == "OrderedDict({'k': R<1>})"
assert repr(defaultdict(list, {"k": [R(1)]})) == "defaultdict(<class 'list'>, {'k': [R<1>]})"

# Named tuples and dataclasses: repr, str, print form and f-strings.
nt = NT(R(1), "s")
assert repr(nt) == "NT(a=R<1>, b='s')"
assert str(nt) == "NT(a=R<1>, b='s')"
assert f"{nt}" == "NT(a=R<1>, b='s')"
assert "%s" % (nt,) == "NT(a=R<1>, b='s')"
assert str(NT(B(), [B()])) == "NT(a=B-repr, b=[B-repr])"
assert repr(NT(1, 2)._replace(b=R(5))) == "NT(a=1, b=R<5>)"
assert repr(NT("a\nb", 1)) == "NT(a='a\\nb', b=1)"
assert repr(DC(R(3))) == "DC(v=R<3>, items=[])"
assert str(DC(B(), [B()])) == "DC(v=B-repr, items=[B-repr])"
assert repr(Money(Decimal("1.50"), date(2024, 5, 1))) == "Money(amount=Decimal('1.50'), day=datetime.date(2024, 5, 1))"
assert repr([NT(Decimal("2"), Color.RED)]) == "[NT(a=Decimal('2'), b=<Color.RED: 1>)]"

# Enum members and their containers.
assert repr([Color.RED, Num.TWO]) == "[<Color.RED: 1>, <Num.TWO: 2>]"
assert str([Num.TWO]) == "[<Num.TWO: 2>]" and str(Num.TWO) == "2"

# str() falls back to __repr__; containers use repr for their elements.
assert str(R(1)) == "R<1>" and f"{R(1)}" == "R<1>" and format(R(1)) == "R<1>"
assert str([S()])[:2] == "[<" and str(S()) == "S!"
assert str({"k": B()}) == "{'k': B-repr}" and str((B(),)) == "(B-repr,)"
assert str({1: R(1)}.values()) == "dict_values([R<1>])"
assert str({1: R(1)}.items()) == "dict_items([(1, R<1>)])"
assert repr(list(itertools.chain([R(1)], [R(2)]))) == "[R<1>, R<2>]"

# Exceptions holding instances.
assert repr(ValueError(R(1))) == "ValueError(R<1>)"
assert str(ValueError(B())) == "B-str"
assert str(ValueError("a", R(1))) == "('a', R<1>)"
assert str(KeyError(R(1))) == "R<1>"
assert repr([ValueError(R(1))]) == "[ValueError(R<1>)]"
try:
    {}[R(9)]
except KeyError as e:
    assert str(e) == "R<9>"
    assert repr(e) == "KeyError(R<9>)"

# Cycles print as Python prints them.
lst = [1]
lst.append(lst)
assert repr(lst) == "[1, [...]]"
d = {}
d["a"] = d
assert str(d) == "{'a': {...}}"
d2 = {}
d2["k"] = [d2]
assert repr(d2) == "{'k': [{...}]}"
t = []
t.append((t,))
assert repr(t) == "[([...],)]"
dq = deque()
dq.append(dq)
assert repr(dq) == "deque([[...]])"
dc = DC(1)
dc.items.append(dc)
assert repr(dc) == "DC(v=1, items=[...])"
nt2 = NT([], 1)
nt2.a.append(nt2)
assert repr(nt2) == "NT(a=[NT(a=[...], b=1)], b=1)"
inner = deque()
outer = [inner]
inner.append(outer)
assert repr(outer) == "[deque([[...]])]"
assert repr(inner) == "deque([[[...]]])"


class Holder:
    def __init__(self):
        self.items = []

    def __repr__(self):
        return "Holder(%r)" % (self.items,)


h = Holder()
h.items.append(h)
assert repr(h) == "Holder([Holder([...])])"
assert str({R(1)}) == "{R<1>}" and repr(frozenset([R(2)])) == "frozenset({R<2>})"

print("OK")
