# Round-19 scattergun: %-formatting, catchable errors, set comparisons,
# unicode predicates, type identity, os.path posixpath semantics, and
# collections gaps. Every value case was diffed against CPython 3.14.

# --- %-formatting -----------------------------------------------------------
assert "%s|%d|%i|%u" % ("a", 5, 5, 5) == "a|5|5|5"
assert "%05.2f" % 3.14159 == "03.14"
assert "%x|%X|%o|%#x" % (255, 255, 8, 255) == "ff|FF|10|0xff"
assert "%e|%E" % (12345.6789, 12345.6789) == "1.234568e+04|1.234568E+04"
assert "%g|%G" % (1000000.0, 0.000001) == "1e+06|1E-06"
assert "%c%c" % (65, "b") == "Ab"
assert "%(n)d items" % {"n": 3} == "3 items"
assert "100%% sure" % () == "100% sure"
try:
    "%d %d" % (1,)
    assert False, "expected TypeError"
except TypeError:
    pass
try:
    "%y" % (1,)
    assert False, "expected ValueError"
except ValueError:
    pass
try:
    "%d" % "x"
    assert False, "expected TypeError"
except TypeError:
    pass

# --- Catchable error types ---------------------------------------------------
try:
    assert 1 == 2, "math is broken"
except AssertionError as e:
    assert str(e) == "math is broken"

try:
    assert False
except AssertionError:
    pass

try:
    "abc".index("z")
except ValueError as e:
    assert "substring not found" in str(e)

s = {1, 2}
s.discard(99)  # no raise
try:
    s.remove(99)
    assert False, "expected KeyError"
except KeyError as e:
    assert "'99'" in str(e) or "99" in str(e)

try:
    max([])
    assert False, "expected ValueError"
except ValueError:
    pass
try:
    min([])
except ValueError:
    pass

try:
    [1].remove(5)
except ValueError:
    pass

import random
try:
    random.choice([])
except IndexError:
    pass
try:
    random.randrange(5, 2)
except ValueError:
    pass

# --- Set comparison operators ------------------------------------------------
assert {1} < {1, 2}
assert {1, 2} <= {1, 2}
assert not ({1, 2} < {1, 2})
assert {1, 2} >= {1}
assert {1, 2} > {1}
assert {1, 2} == {2, 1}
assert {1, 2} != {1, 3}
assert {1, 2} == {1, 2, 3} or {1, 2} != {1, 2, 3}

# --- Unicode string predicates ------------------------------------------------
assert "٥".isdigit() and "٥".isdecimal()  # Arabic-Indic digit (category Nd)
assert "5".isdigit() and not "abc".isdigit()
assert "é".isalpha() and "日本語".isalpha()
assert "\xa0".isspace()

# --- slice attributes ---------------------------------------------------------
sl = slice(1, 5, 2)
assert [1, 2, 3, 4, 5][sl] == [2, 4]
assert (sl.start, sl.stop, sl.step) == (1, 5, 2)
assert slice(None, 3).start is None

# --- type() identity bridges to type builtins ----------------------------------
assert type(5) is int
assert type("") is str
assert type([]) is list
assert type({}) is dict
assert type(()) is tuple
assert type(1.5) is float
assert type(True) is bool
assert not (type(True) is int)  # type(True) is bool, as in Python
assert type(5) == int
assert isinstance(True, int)  # bool subclasses int
assert isinstance(True, bool)
assert type(b"") is bytes

# --- os.path posixpath semantics ----------------------------------------------
import os
assert os.path.basename("/a/b/c.txt") == "c.txt"
assert os.path.basename("/a/b/") == ""
assert os.path.basename("") == ""
assert os.path.basename("/") == ""
assert os.path.dirname("/a/b/c.txt") == "/a/b"
assert os.path.dirname("/a/b/") == "/a/b"
assert os.path.dirname("c.txt") == ""
assert os.path.dirname("") == ""
assert os.path.dirname("///") == "///"
assert os.path.splitext("a.b.c") == ("a.b", ".c")
assert os.path.splitext(".hidden") == (".hidden", "")
assert os.path.splitext("a/.hidden") == ("a/.hidden", "")
assert os.path.splitext("noext") == ("noext", "")
assert os.getpid() > 0

# --- collections gaps ----------------------------------------------------------
d = {"a": 1, "b": 2}
k, v = d.popitem()
assert (k, v) in (("a", 1), ("b", 2)) and len(d) == 1
try:
    {}.popitem()
    assert False, "expected KeyError"
except KeyError:
    pass

from collections import defaultdict
dd = defaultdict(list)
dd["k"].append(1)
assert "k" in dd
assert "z" not in dd
assert len(dd) == 1
dd2 = defaultdict(int)
assert dd2["missing"] == 0

from collections import namedtuple
P = namedtuple("P", ["x", "y"])
p = P(1, 2)
q = p._replace(x=9)
assert repr(q) == "P(x=9, y=2)"
assert p.x == 1  # original untouched
assert p == P(1, 2)  # value equality, as tuples
assert q == P(9, 2)
assert p._asdict()["y"] == 2

# --- __init__ return check -------------------------------------------------------
class BadInit:
    def __init__(self):
        return 5

try:
    BadInit()
    assert False, "expected TypeError"
except TypeError as e:
    assert "__init__() should return None" in str(e)

# --- Property setter errors are AttributeError -----------------------------------
class Prop:
    @property
    def ro(self):
        return 1

pr = Prop()
try:
    pr.ro = 5
    assert False, "expected AttributeError"
except AttributeError as e:
    assert "has no setter" in str(e)

# --- random self-consistency (stream is Go-seeded, not MT19937) -------------------
random.seed(42)
a = [random.randint(0, 99) for _ in range(8)]
random.seed(42)
b = [random.randint(0, 99) for _ in range(8)]
assert a == b
vals = [random.random() for _ in range(50)]
assert all(0 <= x < 1 for x in vals)

print("scatter_round19_test passed")
