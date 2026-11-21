# Gaps found by running whole programs against Python 3.
import re
from datetime import date, datetime, time, timedelta, timezone

# Compiled patterns have sub, subn, split and fullmatch, Python's flags,
# pattern and repr; matches have a repr and groups(default).
p = re.compile(r"(\d)", re.I)
assert p.sub("#", "a1b2") == "a#b#"
assert p.subn(lambda m: m.group(1) * 2, "a1b2") == ("a11b22", 2)
assert p.split("a1b") == ["a", "1", "b"]
assert p.fullmatch("7") is not None and p.fullmatch("77") is None
assert p.pattern == r"(\d)"
assert p.flags == re.I | 32
assert re.compile("(?im)x").flags == re.I | re.M | 32
assert repr(re.compile("a'b", re.M)) == "re.compile(\"a'b\", re.MULTILINE)"
assert repr(re.search("b+", "abbc")) == "<re.Match object; span=(1, 3), match='bb'>"
m = re.match(r"(?P<a>a)(?P<b>b)?", "ac")
assert m.groups() == ("a", None) and m.groups("-") == ("a", "-")
assert m.groupdict(default="") == {"a": "a", "b": ""}

# __init_subclass__ runs for each subclass; super() works in class methods.
registry = []
class Plugin:
    def __init_subclass__(cls, **kw):
        super().__init_subclass__(**kw)
        registry.append(cls.__name__)
    @classmethod
    def make(cls):
        return cls.__name__
class A(Plugin):
    pass
class B(A):
    @classmethod
    def make(cls):
        return "B:" + super().make()
assert registry == ["A", "B"]
assert B.make() == "B:B"
assert [c.__name__ for c in B.__mro__[:3]] == ["B", "A", "Plugin"]

# __slots__ restricts instance attributes unless a base lacks them.
class Slotted:
    __slots__ = ("x",)
s = Slotted()
s.x = 1
try:
    s.y = 2
    assert False, "slots not enforced"
except AttributeError:
    pass
class Open(Slotted):
    pass
Open().y = 2

# isinstance with datetime types; a datetime is a date.
assert isinstance(datetime(2026, 1, 1), date) and not isinstance(date(2026, 1, 1), datetime)
assert isinstance(timedelta(1), timedelta) and isinstance(time(1), time)
assert isinstance(timezone.utc, timezone) and not isinstance(1, date)

# Starred targets alongside nested ones, in comprehensions, trailing commas.
first, (second, third), *rest = [1, (2, 3), 4, 5]
assert (first, second, third, rest) == (1, 2, 3, [4, 5])
assert [b for *a, b in [(1, 2, 3), (4, 5)]] == [3, 5]
*only, = [1, 2]
assert only == [1, 2]
assert [c for c, in [(5,)]] == [5]
for (d,) in [(6,)]:
    assert d == 6

# Chained assignment binds left to right.
k, l = l, k = 1, 2
assert (k, l) == (2, 1)

# t[0] += [x] on a tuple changes the list, then raises.
t = ([1], 2)
try:
    t[0] += [3]
    assert False, "tuple item assignment allowed"
except TypeError:
    pass
assert t == ([1, 3], 2)

# StopIteration from next() has no message; exception args are not in vars().
try:
    next(iter([]))
except StopIteration as e:
    assert str(e) == ""
class NotFound(Exception):
    def __init__(self, key):
        super().__init__(key)
        self.key = key
assert vars(NotFound("q")) == {"key": "q"}

# A match is subscriptable: m[g] is m.group(g).
mm = re.search(r"(?P<x>a)(b)?", "xa")
assert (mm[0], mm[1], mm["x"], mm[2]) == ("a", "a", "a", None)
