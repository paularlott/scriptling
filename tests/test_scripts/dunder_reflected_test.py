# Reflected and in-place operator dunders, __reversed__ and setattr() follow
# Python 3: this script also runs unchanged under CPython 3.14.
import dataclasses

class V:
    def __init__(self, x):
        self.x = x
    def __add__(self, o):
        return V(self.x + (o.x if isinstance(o, V) else o))
    def __radd__(self, o):
        return V(o + self.x)
    def __rmul__(self, o):
        return V(o * self.x * 10)
    def __rsub__(self, o):
        return V(o - self.x)

assert (1 + V(2)).x == 3
assert (3 * V(2)).x == 60
assert (10 - V(3)).x == 7
assert (V(1) + V(2)).x == 3

class W:
    def __radd__(self, o):
        return "W"
assert "a" + W() == "W"
assert (V(1) + W()).x == "W"

class Acc:
    def __init__(self):
        self.items = []
    def __iadd__(self, o):
        self.items.append(o)
        return self
a = Acc()
b = a
a += 5
assert a is b and a.items == [5]

class Plain:
    def __init__(self, x):
        self.x = x
    def __add__(self, o):
        return Plain(self.x + o)
p = Plain(1)
q = p
p += 2
assert p is not q and p.x == 3 and q.x == 1

class R:
    def __reversed__(self):
        return iter([3, 2, 1])
assert list(reversed(R())) == [3, 2, 1]

class P:
    @property
    def ro(self):
        return 1
    @property
    def rw(self):
        return self._v
    @rw.setter
    def rw(self, v):
        self._v = v * 2
o = P()
try:
    setattr(o, "ro", 5)
    assert False, "setattr on a read-only property"
except AttributeError:
    pass
setattr(o, "rw", 5)
assert o.rw == 10

@dataclasses.dataclass(frozen=True)
class F:
    a: int
try:
    setattr(F(1), "a", 2)
    assert False, "setattr on a frozen dataclass"
except AttributeError:
    pass

class Never:
    def __eq__(self, o):
        return False
n = Never()
assert n != n
assert [n] == [n] and (n,) == (n,) and {"k": n} == {"k": n}
assert n in [n] and [n, n].count(n) == 2
