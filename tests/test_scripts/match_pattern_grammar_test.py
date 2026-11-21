# The match statement's pattern grammar, as Python's: "as" after any
# sub-pattern, or-patterns and groups at any depth, star and double-star
# captures, value patterns, literals, class patterns and guards.
import math as mod
from collections import namedtuple
from dataclasses import dataclass
from enum import Enum, IntEnum, StrEnum


class Status(Enum):
    PAID = 1
    DUE = 2


class Code(IntEnum):
    OK = 200


class Col(StrEnum):
    RED = "red"


@dataclass
class Pt:
    x: int
    y: int


NT = namedtuple("NT", "a b")


class Plain:
    __match_args__ = ("u", "v")

    def __init__(self, u, v):
        self.u, self.v = u, v


def m(s):
    match s:
        case {"req": {"status": ("404" | "410") as code}}:
            return ("gone", code)
        case {"req": {"status": str() as st, **rest}}:
            return ("st", st, rest)
        case [1, *rest, (2 | 3) as last]:
            return ("seq", rest, last)
        case [*_, 99]:
            return "ends99"
        case (0, 0) | [0, 0, 0]:
            return "zeros"
        case Pt(0, y) if y > 0:
            return ("pt0", y)
        case Pt(x=x, y=0) as p:
            return ("ptx", x, p.x)
        case NT(a, b=[_, *more]):
            return ("nt", a, more)
        case Status.PAID:
            return "paid"
        case Code.OK:
            return "ok200"
        case "red":
            return "red-literal"
        case -1 | -2.5:
            return "neg"
        case 1.5:
            return "float"
        case None:
            return "none"
        case True:
            return "true"
        case mod.pi:
            return "pi"
        case Plain(1, v=(2 | 3) as vv):
            return ("plain", vv)
        case Plain(u, v):
            return ("plain2", u, v)
        case {"k": [{"x": int(n)} as inner]}:
            return ("deep", n, inner)
        case ((a, b), (c, d)):
            return ("nested", a, b, c, d)
        case [a, [b, [c, *d]]]:
            return ("nest3", a, b, c, d)
        case {1: one, "two": two} if one < two:
            return ("mixkeys", one, two)
        case str(s2) if len(s2) > 5:
            return ("long", s2)
        case int() | float() as num:
            return ("num", num)
        case b"xy":
            return "bytes"
        case {}:
            return "emptymap"
        case []:
            return "emptyseq"
        case _:
            return "other"


cases = [
    ({"req": {"status": "404"}}, ("gone", "404")),
    ({"req": {"status": "410", "x": 1}}, ("gone", "410")),
    ({"req": {"status": "500", "y": 2}}, ("st", "500", {"y": 2})),
    ([1, 2, 3], ("seq", [2], 3)),
    ([1, 3], ("seq", [], 3)),
    ([1, 5, 6, 2], ("seq", [5, 6], 2)),
    ([5, 99], "ends99"),
    ([99], "ends99"),
    ((0, 0), "zeros"),
    ([0, 0, 0], "zeros"),
    ([0, 0], "zeros"),
    (Pt(0, 5), ("pt0", 5)),
    (Pt(3, 0), ("ptx", 3, 3)),
    (Pt(0, 0), ("ptx", 0, 0)),
    (NT(1, [2, 3, 4]), ("nt", 1, [3, 4])),
    (NT(1, []), "other"),
    (Status.PAID, "paid"),
    (Status.DUE, "other"),
    (Code.OK, "ok200"),
    (200, "ok200"),
    (Col.RED, "red-literal"),
    ("red", "red-literal"),
    (-1, "neg"),
    (-2.5, "neg"),
    (1.5, "float"),
    (None, "none"),
    (True, "true"),
    (False, ("num", False)),
    (1, ("num", 1)),
    (3.141592653589793, "pi"),
    (Plain(1, 2), ("plain", 2)),
    (Plain(1, 4), ("plain2", 1, 4)),
    ({"k": [{"x": 5}]}, ("deep", 5, {"x": 5})),
    (((1, 2), (3, 4)), ("nested", 1, 2, 3, 4)),
    ([1, [2, [3, 4, 5]]], ("nest3", 1, 2, 3, [4, 5])),
    ({1: 1, "two": 2}, ("mixkeys", 1, 2)),
    ({1: 3, "two": 2}, "emptymap"),
    ("abcdefg", ("long", "abcdefg")),
    ("abc", "other"),
    (7, ("num", 7)),
    (2.0, ("num", 2.0)),
    (b"xy", "bytes"),
    ({}, "emptymap"),
    ([], "emptyseq"),
    ((), "emptyseq"),
]
for subject, want in cases:
    got = m(subject)
    assert got == want, (subject, got, want)


def star(s):
    match s:
        case a, *b:
            return a, b
        case _:
            return None


assert star([1, 2, 3]) == (1, [2, 3])
assert star("ab") is None
assert star((5,)) == (5, [])


def as_in_sequence(s):
    match s:
        case [x, y] as whole if x < y:
            return whole
        case a, b as c:
            return (a, b, c)
    return "no"


assert as_in_sequence([1, 2]) == [1, 2]
assert as_in_sequence([2, 1]) == (2, 1, 1)


def groups(s):
    match s:
        case ((1)):
            return "one"
        case (((2 | 3) as t),):
            return ("t", t)
    return "no"


assert groups(1) == "one"
assert groups((3,)) == ("t", 3)
assert groups([2]) == ("t", 2)
assert groups(4) == "no"


def nested_or(s):
    match s:
        case {"a": [1 | 2, (3 | 4) as y] | (5, y)}:
            return y
    return None


assert nested_or({"a": [1, 4]}) == 4
assert nested_or({"a": (5, 9)}) == 9
assert nested_or({"a": [3, 3]}) is None


def star_middle(s):
    match s:
        case [first, *middle, last]:
            return (first, middle, last)
        case [*_]:
            return "short"


assert star_middle([1, 2, 3, 4]) == (1, [2, 3], 4)
assert star_middle([1]) == "short"


def kw_class(s):
    match s:
        case Pt(y=1 | 2 as yy, x=xx):
            return (xx, yy)
    return None


assert kw_class(Pt(5, 2)) == (5, 2)
assert kw_class(Pt(5, 3)) is None

print("OK")
