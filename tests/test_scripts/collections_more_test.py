# Counter arithmetic and comparisons, namedtuple as a tuple.
from collections import Counter, namedtuple

# --- Counter
C = Counter
assert +C(a=2, b=-1, c=0) == C(a=2)
assert -C(a=2, b=-1) == C(b=1)
assert C(a=2) + C(a=1, b=1) == C(a=3, b=1)
assert C(a=2) - C(a=3, b=1) == C()
assert C(a=2, b=1) & C(a=1, c=3) == C(a=1)
assert C(a=2, b=1) | C(a=1, c=3) == C(a=2, b=1, c=3)
assert C(a=1) <= C(a=2) and C(a=1) < C(a=2)
assert not C(a=1) < C(a=1) and C(a=1) <= C(a=1)
assert C(a=1) >= C() and C(a=1) > C()
assert C(a=-1) < C()
assert C(a=1) == C(a=1, b=0)
assert C(a=1, b=3).total() == 4 and C().total() == 0
assert list(C("abbccc").elements()) == ["a", "b", "b", "c", "c", "c"]
assert list(C(b=2, a=1).elements()) == ["b", "b", "a"]
assert list(C(a=0, b=-1, c=2).elements()) == ["c", "c"]
c = C(a=2)
c.subtract(a=5)
assert c == C(a=-3) and c["a"] == -3
c.subtract("aa")
assert c["a"] == -5
c = C(a=1)
c += C(a=2, b=-1)
assert c == C(a=3)
c |= C(b=4)
assert c == C(a=3, b=4)
c &= C(a=1, b=9)
assert c == C(a=1, b=4)
try:
    C(a=1) <= 5
    assert False
except TypeError:
    pass
try:
    C(a=1) + {"a": 1}
    assert False
except TypeError as e:
    assert "unsupported operand" in str(e)

# --- namedtuple is a tuple
P = namedtuple("P", "x y")
p = P(1, 2)
assert isinstance(p, tuple) and issubclass(P, tuple)
assert hash(p) == hash((1, 2))
assert p == (1, 2) and (1, 2) == p and (1, 3) != p
assert p < (1, 3) and (0, 9) < p
x, *rest = p
assert x == 1 and rest == [2]
assert sorted([P(2, 1), P(1, 5)]) == [(1, 5), (2, 1)]

class Q(P):
    def total(self):
        return self.x + self.y

q = Q(3, 4)
assert q.total() == 7 and isinstance(q, tuple) and isinstance(q, P)
assert repr(q) == "Q(x=3, y=4)"
assert repr(q._replace(x=0)) == "Q(x=0, y=4)"

print("OK")
