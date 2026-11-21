# Sets of every size from empty to well past the point where a small set
# changes its storage, exercising each operation on both sides of it. Results
# are compared with sorted() or set equality, never by iteration order.
# Everything here behaves the same under CPython 3.

def expect(exc, fn):
    try:
        fn()
    except exc:
        return True
    except Exception as e:
        return "wrong: " + type(e).__name__
    return "no error"

for n in range(21):
    items = list(range(n))

    # building
    s = set()
    for i in items:
        s.add(i)
    assert len(s) == n
    assert sorted(s) == items
    assert set(items) == s
    assert {i for i in range(n)} == s
    assert set(range(n)) == s
    assert set(items + items) == s
    assert bool(s) == (n > 0)
    for i in items:
        assert i in s
    assert n not in s
    assert -1 not in s
    assert "0" not in s

    # adding an existing element changes nothing
    s.add(0)
    assert len(s) == max(n, 1)
    if n == 0:
        s.discard(0)

    # copy is independent
    c = s.copy()
    assert c == s
    c.add(1000)
    assert 1000 not in s
    assert len(c) == n + 1

    # algebra against an overlapping set
    t = set(range(n // 2, n + 5))
    assert sorted(s | t) == list(range(n + 5))
    assert sorted(s & t) == list(range(n // 2, n))
    assert sorted(s - t) == list(range(n // 2))
    assert sorted(s ^ t) == sorted(set(range(n // 2)) | set(range(n, n + 5)))
    assert s.union(t) == s | t
    assert s.intersection(t) == s & t
    assert s.difference(t) == s - t
    assert s.symmetric_difference(t) == s ^ t
    assert s.union(t, [100, 101]) == s | t | {100, 101}

    # in-place forms
    u = s.copy()
    u |= t
    assert u == s | t
    u = s.copy()
    u &= t
    assert u == s & t
    u = s.copy()
    u -= t
    assert u == s - t
    u = s.copy()
    u ^= t
    assert u == s ^ t
    u = s.copy()
    u.update(t)
    assert u == s | t
    u = s.copy()
    u.intersection_update(t)
    assert u == s & t
    u = s.copy()
    u.difference_update(t)
    assert u == s - t
    u = s.copy()
    u.symmetric_difference_update(t)
    assert u == s ^ t

    # with itself
    u = s.copy()
    u |= u
    assert u == s
    u &= u
    assert u == s
    u ^= u
    assert u == set()
    u = s.copy()
    u -= u
    assert u == set()

    # subsets, supersets and comparisons
    half = set(range(n // 2))
    assert half <= s
    assert s >= half
    assert half.issubset(s)
    assert s.issuperset(half)
    assert (half < s) == (len(half) < n)
    assert (s > half) == (len(half) < n)
    assert s <= s and s >= s
    assert not (s < s) and not (s > s)
    assert s == set(reversed(items))
    assert (s != t) == (s != set(range(n // 2, n + 5)))
    assert s.isdisjoint({-1, -2})
    assert s.isdisjoint(t) == (n == 0 or len(s & t) == 0)

    # removal
    r = s.copy()
    for i in items:
        if i % 2 == 0:
            r.remove(i)
    assert sorted(r) == [i for i in items if i % 2 == 1]
    for i in items:
        r.discard(i)
        r.discard(i)
    assert r == set()
    assert expect(KeyError, lambda: r.remove(5)) == True

    # pop drains the set, every element exactly once
    p = s.copy()
    popped = []
    while p:
        popped.append(p.pop())
    assert sorted(popped) == items
    assert expect(KeyError, lambda: p.pop()) == True

    # clear, then reuse
    q = s.copy()
    q.clear()
    assert len(q) == 0 and q == set()
    q.add("x")
    assert q == {"x"}

    # removing while iterating over a list copy
    w = s.copy()
    for v in list(w):
        if v % 3 == 0:
            w.remove(v)
    assert sorted(w) == [i for i in items if i % 3 != 0]

    # frozenset
    f = frozenset(items)
    assert len(f) == n
    assert f == s
    assert sorted(f) == items
    assert frozenset(reversed(items)) == f
    assert hash(frozenset(reversed(items))) == hash(f)
    assert {f: 1}[frozenset(items)] == 1
    assert type(f | {99}) == frozenset
    assert type(s | f) == set
    assert expect(AttributeError, lambda: f.add(1)) == True

# grow past the threshold, shrink back, grow again
g = set()
for i in range(30):
    g.add(i)
    assert len(g) == i + 1
for i in range(29, 2, -1):
    g.remove(i)
    assert len(g) == i
assert g == {0, 1, 2}
for i in range(3, 25):
    g.add(i * 10)
assert len(g) == 25
assert 230 in g and 3 not in g

# mixed element types and Python's equal-hash rules
m = {1, "1", 1.5, None, (1, 2), "a", 2.0, True, False, 0}
assert len(m) == 8
assert 1 in m and True in m and 1.0 in m
assert 2 in m and 2.0 in m
assert 0 in m and False in m
assert None in m
assert (1, 2) in m and (2, 1) not in m
assert "1" in m and "2" not in m
m.discard(True)
assert 1 not in m and 1.0 not in m
m.discard(0.0)
assert False not in m
assert len(m) == 6

big_mixed = set()
for i in range(15):
    big_mixed.add(i)
    big_mixed.add(str(i))
    big_mixed.add((i, str(i)))
assert len(big_mixed) == 45
assert 7.0 in big_mixed and "7" in big_mixed and (7, "7") in big_mixed
assert (7, 7) not in big_mixed

# from strings
assert set("hello") == {"h", "e", "l", "o"}
assert len(set("abcdefghijklmnopqrstuvwxyz" * 2)) == 26
assert sorted(set("mississippi")) == ["i", "m", "p", "s"]

# user classes with __hash__ and __eq__
class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y
    def __hash__(self):
        return hash((self.x, self.y))
    def __eq__(self, other):
        return isinstance(other, Point) and self.x == other.x and self.y == other.y

for n in [1, 3, 8, 9, 16]:
    ps = set()
    for i in range(n):
        ps.add(Point(i, -i))
        ps.add(Point(i, -i))
    assert len(ps) == n
    assert Point(n - 1, -(n - 1)) in ps
    assert Point(n, -n) not in ps
    ps.remove(Point(0, 0))
    assert len(ps) == n - 1
    assert Point(0, 0) not in ps
    ps.discard(Point(0, 0))
    assert len(ps) == n - 1
    other = {Point(i, -i) for i in range(n)}
    assert len(other - ps) == 1
    assert len(ps | other) == n
    assert ps <= other

# comprehensions with duplicates and conditions
assert {i % 5 for i in range(100)} == {0, 1, 2, 3, 4}
assert {i for i in range(20) if i % 2} == set(range(1, 20, 2))
assert len({(i, j) for i in range(4) for j in range(4)}) == 16

# literals across sizes, with duplicates
assert len({1, 1, 1}) == 1
assert len({1, 2, 3, 4, 5, 6, 7, 8, 8, 8}) == 8
assert len({1, 2, 3, 4, 5, 6, 7, 8, 9, 9}) == 9
assert {*range(12), *range(6, 20)} == set(range(20))
