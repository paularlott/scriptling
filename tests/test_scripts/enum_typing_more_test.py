# enum functional API, @unique and _missing_; typing NamedTuple classes,
# generic aliases and unions at run time.
import enum
import typing
from typing import NamedTuple, Optional, Union

Color = enum.Enum("Color", "RED GREEN")
assert [m.name for m in Color] == ["RED", "GREEN"]
assert Color.RED.value == 1 and Color(2) is Color.GREEN and Color["RED"] is Color.RED
assert Color.__name__ == "Color" and repr(Color) == "<enum 'Color'>"
assert repr(Color.RED) == "<Color.RED: 1>" and str(Color.RED) == "Color.RED"
assert [m.value for m in enum.Enum("C2", "A, B,C")] == [1, 2, 3]
assert [m.value for m in enum.Enum("C3", ["X", "Y"], start=5)] == [5, 6]
assert [m.value for m in enum.Enum("C4", [("P", "p"), ("Q", "q")])] == ["p", "q"]
assert [m.value for m in enum.Enum("C5", {"M": 10, "N": 20})] == [10, 20]
I = enum.IntEnum("I", "A B")
assert I.B == 2 and isinstance(I.A, enum.Enum)
F = enum.Flag("F", "R W X")
assert [m.value for m in F] == [1, 2, 4]
S = enum.StrEnum("S", "Ab Cd")
assert S.Ab == "ab"
assert len(enum.Enum("L", "A B C")) == 3

@enum.unique
class U(enum.Enum):
    A = 1
    B = 2

assert [m.name for m in U] == ["A", "B"]
try:
    @enum.unique
    class V(enum.Enum):
        A = 1
        B = 1
        C = 2
        D = 2
    assert False
except ValueError as e:
    assert str(e) == "duplicate values found in <enum 'V'>: B -> A, D -> C"

class M(enum.Enum):
    A = 1
    B = 2
    @classmethod
    def _missing_(cls, value):
        if value == 0:
            return cls.A
        return None

assert M(0) is M.A and M(2) is M.B
try:
    M(5)
    assert False
except ValueError as e:
    assert str(e) == "5 is not a valid M"

# typing.NamedTuple class syntax
class P(NamedTuple):
    x: int
    y: int = 0
    def s(self):
        return self.x + self.y

assert repr(P(1)) == "P(x=1, y=0)" and P(1, 2).s() == 3
assert P._fields == ("x", "y") and P._field_defaults == {"y": 0}
assert isinstance(P(1), tuple) and P(y=3, x=1) == (1, 3)
assert hash(P(1, 2)) == hash((1, 2))
try:
    class Bad(NamedTuple):
        x: int = 1
        y: int
    assert False
except TypeError as e:
    assert "cannot follow default field" in str(e)
P2 = NamedTuple("P2", [("a", int), ("b", str)])
assert repr(P2(1, "x")) == "P2(a=1, b='x')"

# generic aliases and unions
assert repr(list[int]) == "list[int]" and repr(dict[str, int]) == "dict[str, int]"
assert list[int]([1, 2]) == [1, 2] and list[int].__origin__ is list
assert repr(tuple[int, ...]) == "tuple[int, ...]"
assert repr(int | None) == "int | None"
assert isinstance(3, int | None) and isinstance(None, int | None) and not isinstance("a", int | None)
assert isinstance(1.0, (int, float | str)) and issubclass(bool, str | int)
assert repr(int | str | None) == "int | str | None"
assert repr(Optional[int]) == "int | None" and repr(Union[int, str]) == "int | str"
assert isinstance(3, Optional[int]) and not isinstance([], Optional[int])
assert repr(Union[int]) == "<class 'int'>"

def f(a: int | None = None, b: list[int] = [], c: dict[str, int] | None = None) -> list[str] | None:
    return [str(a)]

assert f(1) == ["1"]
print("OK")
