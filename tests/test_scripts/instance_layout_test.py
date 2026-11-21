# Instance attribute storage: attributes set in any order, number and way,
# deleted and re-added, listed in assignment order by vars() and __dict__.
# Everything here behaves the same under CPython 3.
import copy
from dataclasses import dataclass

class Empty:
    pass

# --- 0 to 20 attributes, in __init__ and after construction ---
for n in range(21):
    o = Empty()
    for k in range(n):
        setattr(o, "a" + str(k), k * 10)
    assert len(vars(o)) == n
    assert list(vars(o)) == ["a" + str(k) for k in range(n)]
    for k in range(n):
        assert getattr(o, "a" + str(k)) == k * 10
    assert not hasattr(o, "a" + str(n))
    assert getattr(o, "a" + str(n), "dflt") == "dflt"
    # overwrite keeps position
    if n > 0:
        setattr(o, "a0", -1)
        assert list(vars(o))[0] == "a0" and o.a0 == -1

class Init:
    def __init__(self, n):
        for k in range(n):
            setattr(self, "f" + str(k), k)

for n in range(21):
    o = Init(n)
    assert list(vars(o).items()) == [("f" + str(k), k) for k in range(n)]

# --- conditional attributes: instances of one class with different layouts ---
class Cond:
    def __init__(self, i):
        if i % 2:
            self.odd = True
        self.i = i
        if i % 3 == 0:
            self.three = i // 3
        else:
            self.other = "x"

objs = [Cond(i) for i in range(12)]
for i, o in enumerate(objs):
    want = []
    if i % 2:
        want.append("odd")
    want.append("i")
    want.append("three" if i % 3 == 0 else "other")
    assert list(vars(o)) == want, (i, list(vars(o)))
    assert o.i == i

# different orders of the same names
p, q = Empty(), Empty()
p.x = 1
p.y = 2
q.y = 3
q.x = 4
assert list(vars(p)) == ["x", "y"] and list(vars(q)) == ["y", "x"]
assert (p.x, p.y, q.x, q.y) == (1, 2, 4, 3)

# --- deletion and re-adding ---
o = Empty()
for k in range(8):
    setattr(o, "d" + str(k), k)
del o.d0
delattr(o, "d5")
assert list(vars(o)) == ["d1", "d2", "d3", "d4", "d6", "d7"]
assert o.d6 == 6 and o.d7 == 7 and o.d1 == 1
o.d0 = "back"
assert list(vars(o))[-1] == "d0" and o.d0 == "back"
try:
    del o.d5
    assert False, "deleting a missing attribute must raise"
except AttributeError:
    pass
try:
    o.d5
    assert False
except AttributeError:
    pass
# delete everything then rebuild
for name in list(vars(o)):
    delattr(o, name)
assert vars(o) == {}
o.z = 1
assert vars(o) == {"z": 1}

# deletion on a large instance
big = Empty()
for k in range(40):
    setattr(big, "b" + str(k), k)
for k in range(0, 40, 3):
    delattr(big, "b" + str(k))
assert list(vars(big)) == ["b" + str(k) for k in range(40) if k % 3]
for k in range(40):
    assert getattr(big, "b" + str(k), None) == (None if k % 3 == 0 else k)
big.b0 = "new"
assert list(vars(big))[-1] == "b0"

# --- __dict__ is in order and writes through ---
o = Empty()
o.a = 1
o.b = 2
d = o.__dict__
assert list(d) == ["a", "b"] and d["a"] == 1
o.__dict__["c"] = 3
assert o.c == 3 and list(vars(o)) == ["a", "b", "c"]
o.__dict__["a"] = 10
assert o.a == 10 and list(vars(o)) == ["a", "b", "c"]
del o.__dict__["b"]
assert not hasattr(o, "b") and list(vars(o)) == ["a", "c"]

# --- class attributes shadowed and un-shadowed ---
class Shadow:
    kind = "class"
    def meth(self):
        return "method"

s = Shadow()
assert s.kind == "class"
s.kind = "instance"
assert s.kind == "instance" and Shadow.kind == "class"
del s.kind
assert s.kind == "class"
s.meth = lambda: "attr"
assert s.meth() == "attr"
del s.meth
assert s.meth() == "method"

# --- properties and __getattr__ (__setattr__ is not supported) ---
class Prop:
    def __init__(self):
        self._v = 1
    @property
    def v(self):
        return self._v * 2
    @v.setter
    def v(self, x):
        self._v = x

p = Prop()
assert p.v == 2
p.v = 5
assert p.v == 10 and list(vars(p)) == ["_v"]

class Fallback:
    def __init__(self):
        self.real = 1
    def __getattr__(self, name):
        return "missing:" + name

f = Fallback()
assert f.real == 1 and f.other == "missing:other"
f.other = 2
assert f.other == 2
del f.other
assert f.other == "missing:other"

# --- inheritance ---
class Base:
    def __init__(self):
        self.base1 = 1
        self.base2 = 2

class Child(Base):
    def __init__(self):
        super().__init__()
        self.child = 3

class Other(Base):
    def __init__(self):
        self.first = 0
        super().__init__()

c = Child()
assert list(vars(c)) == ["base1", "base2", "child"]
o2 = Other()
assert list(vars(o2)) == ["first", "base1", "base2"]
b = Base()
assert list(vars(b)) == ["base1", "base2"]

# --- two classes with the same attribute names ---
class One:
    def __init__(self):
        self.x = "one-x"
        self.y = "one-y"

class Two:
    def __init__(self):
        self.y = "two-y"
        self.x = "two-x"

one, two = One(), Two()
assert (one.x, one.y, two.x, two.y) == ("one-x", "one-y", "two-x", "two-y")
assert list(vars(one)) == ["x", "y"] and list(vars(two)) == ["y", "x"]
del one.x
assert two.x == "two-x" and list(vars(two)) == ["y", "x"]

# --- dataclasses ---
@dataclass
class DC:
    x: int
    y: int = 0

d1 = DC(1, y=2)
assert (d1.x, d1.y) == (1, 2) and list(vars(d1)) == ["x", "y"]
d1.z = 3
assert list(vars(d1)) == ["x", "y", "z"]
assert DC(1, 2) == DC(1, 2)

# --- copy ---
src = Empty()
src.a = [1, 2]
src.b = "s"
for k in range(10):
    setattr(src, "c" + str(k), k)
del src.c3
sh = copy.copy(src)
dp = copy.deepcopy(src)
assert list(vars(sh)) == list(vars(src)) == list(vars(dp))
assert sh.a is src.a and dp.a is not src.a and dp.a == src.a
dp.b = "changed"
assert src.b == "s"
sh.new = 1
assert not hasattr(src, "new")

# --- instances as dict values and in sets ---
class Key:
    def __init__(self, k):
        self.k = k
    def __hash__(self):
        return hash(self.k)
    def __eq__(self, other):
        return isinstance(other, Key) and self.k == other.k

table = {i: Init(i % 5) for i in range(10)}
assert all(len(vars(table[i])) == i % 5 for i in range(10))
keys = {Key(1), Key(2), Key(1)}
assert len(keys) == 2 and Key(2) in keys

print("OK")
