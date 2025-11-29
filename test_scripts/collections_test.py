# Test collections library
import collections

# Test Counter
c = collections.Counter([1, 1, 2, 3, 3, 3])
assert c[1] == 2
assert c[2] == 1
assert c[3] == 3

# Test Counter with string
c = collections.Counter("hello")
assert c["l"] == 2
assert c["h"] == 1
assert c["e"] == 1
assert c["o"] == 1

# Test most_common
c = collections.Counter([1, 1, 2, 3, 3, 3])
mc = collections.most_common(c, 2)
assert len(mc) == 2
assert mc[0][0] == 3
assert mc[0][1] == 3
assert mc[1][0] == 1
assert mc[1][1] == 2

# Test most_common with different input
c2 = collections.Counter([1, 1, 2])
mc2 = c2.most_common()
assert len(mc2) == 2
assert mc2[0][0] == 1
assert mc2[0][1] == 2
assert mc2[1][0] == 2
assert mc2[1][1] == 1

# Test OrderedDict
od = collections.OrderedDict([("a", 1), ("b", 2)])
assert od["a"] == 1
assert od["b"] == 2

# Test deque
d = collections.deque([1, 2, 3])
assert len(d) == 3
assert d[0] == 1
assert d[2] == 3

# Test deque_appendleft
d = collections.deque([1, 2, 3])
collections.deque_appendleft(d, 0)
assert d[0] == 0
assert len(d) == 4

# Test deque_popleft
d = collections.deque([1, 2, 3])
x = collections.deque_popleft(d)
assert x == 1
assert len(d) == 2
assert d[0] == 2

# Test deque_rotate
d = collections.deque([1, 2, 3, 4])
collections.deque_rotate(d, 1)
assert d[0] == 4
assert d[1] == 1

d = collections.deque([1, 2, 3, 4])
collections.deque_rotate(d, -1)
assert d[0] == 2
assert d[3] == 1

# Test namedtuple
Point = collections.namedtuple("Point", ["x", "y"])
p = Point(1, 2)
assert p["x"] == 1
assert p["y"] == 2
# Test direct attribute access
assert p.x == 1
assert p.y == 2

# Test DefaultDict (using defaultdict alias which is now the standard)
dd = collections.defaultdict(list)
dd["key"].append(1)
assert dd["key"] == [1]
assert len(dd["key"]) == 1

dd2 = collections.defaultdict(int)
dd2["count"] = dd2["count"] + 1
assert dd2["count"] == 1

# Test DefaultDict alias
dd3 = collections.DefaultDict(list)
dd3["key"].append(1)
assert dd3["key"] == [1]

# Test ChainMap
d1 = {"a": 1}
d2 = {"b": 2, "a": 10}
cm = collections.ChainMap(d1, d2)
assert cm["a"] == 1  # First dict has priority
assert cm["b"] == 2

True
