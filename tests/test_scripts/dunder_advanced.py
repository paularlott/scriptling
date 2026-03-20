# __getitem__ — bracket read access
class Bag:
    def __init__(self, items):
        self.items = items

    def __getitem__(self, idx):
        return self.items[idx]

    def __len__(self):
        return len(self.items)

b = Bag([10, 20, 30])
assert b[0] == 10
assert b[1] == 20
assert b[-1] == 30
assert len(b) == 3

# dot access still works (does NOT call __getitem__)
assert b.items == [10, 20, 30]

# __setitem__ — bracket write access
class MutableBag:
    def __init__(self, items):
        self.items = list(items)

    def __getitem__(self, idx):
        return self.items[idx]

    def __setitem__(self, idx, val):
        self.items[idx] = val

    def __len__(self):
        return len(self.items)

m = MutableBag([1, 2, 3])
m[1] = 99
assert m[1] == 99
assert m[0] == 1
assert m[2] == 3

# dot assignment still goes to Fields (does NOT call __setitem__)
m.extra = "hello"
assert m.extra == "hello"

# __getitem__ with string keys (dict-like)
class Config:
    def __init__(self):
        self.data = {"host": "localhost", "port": 8080}

    def __getitem__(self, key):
        return self.data[key]

    def __setitem__(self, key, val):
        self.data[key] = val

c = Config()
assert c["host"] == "localhost"
assert c["port"] == 8080
c["port"] = 9090
assert c["port"] == 9090

# __hash__ — instances usable as dict keys and set elements
class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __hash__(self):
        return self.x * 31 + self.y

    def __eq__(self, other):
        return self.x == other.x and self.y == other.y

p1 = Point(1, 2)
p2 = Point(1, 2)  # equal to p1
p3 = Point(3, 4)

# hash() calls __hash__
assert hash(p1) == 33
assert hash(p1) == hash(p2)
assert hash(p1) != hash(p3)

# as dict key
d = {}
d[p1] = "origin"
assert d[p1] == "origin"
assert d[p2] == "origin"  # same hash+eq

d[p3] = "other"
assert len(d) == 2

# in operator on dict
assert p1 in d
assert p2 in d
assert p3 in d

p4 = Point(9, 9)
assert p4 not in d

# in set
s = {p1, p2, p3}
assert len(s) == 2  # p1 and p2 deduplicate

s.add(p2)
assert len(s) == 2  # still 2

# unhashable instance (no __hash__) still raises TypeError in set
class NoHash:
    pass

try:
    bad = {NoHash()}
    assert False, "expected TypeError"
except TypeError:
    pass

print("Dunder advanced tests passed!")
