# Test Set type

# Creation
s = set([1, 2, 3])
assert len(s) == 3
assert type(s) == "SET"

s2 = set([1, 2, 2, 3])
assert len(s2) == 3 # Uniqueness

# Iteration
l = []
for x in s:
    l.append(x)
l.sort()
assert l == [1, 2, 3]

# 'in' operator
assert 1 in s
assert 4 not in s

# List conversion
l2 = list(s)
l2.sort()
assert l2 == [1, 2, 3]

# Tuple conversion
t = tuple(s)
assert len(t) == 3

# Set from other iterables
s3 = set("hello")
assert len(s3) == 4 # h, e, l, o
assert "h" in s3
assert "o" in s3

d = {"a": 1, "b": 2}
s4 = set(d.keys())
assert len(s4) == 2
assert "a" in s4
assert "b" in s4

# Set methods
s = set([1, 2])
s.add(3)
assert len(s) == 3
assert 3 in s

s.remove(2)
assert len(s) == 2
assert 2 not in s

s.discard(99) # Should not error
assert len(s) == 2

popped = s.pop()
assert len(s) == 1

s.clear()
assert len(s) == 0

# Set operations
s1 = set([1, 2, 3])
s2 = set([3, 4, 5])

u = s1.union(s2)
assert len(u) == 5
assert 1 in u
assert 5 in u

i = s1.intersection(s2)
assert len(i) == 1
assert 3 in i

diff = s1.difference(s2)
assert len(diff) == 2
assert 1 in diff
assert 2 in diff
assert 3 not in diff

sym_diff = s1.symmetric_difference(s2)
assert len(sym_diff) == 4
assert 3 not in sym_diff

# Subset/Superset
assert s1.issubset(u)
assert u.issuperset(s1)

print("Set tests passed!")

# Set comprehensions
sc = {x for x in [1, 2, 3]}
assert len(sc) == 3
assert 1 in sc
assert 2 in sc
assert 3 in sc

# With filter condition
sc2 = {x for x in [1, 2, 3, 4, 5] if x % 2 == 0}
assert len(sc2) == 2
assert 2 in sc2
assert 4 in sc2
assert 1 not in sc2

# Deduplication
sc3 = {x % 3 for x in [0, 1, 2, 3, 4, 5]}
assert len(sc3) == 3
assert 0 in sc3
assert 1 in sc3
assert 2 in sc3

# From string
sc4 = {c for c in "hello"}
assert len(sc4) == 4  # h, e, l, o
assert "h" in sc4
assert "l" in sc4

print("Set comprehension tests passed!")

# Hashability: unhashable types must raise TypeError
try:
    s = set([[1, 2]])
    assert False, "expected TypeError for list in set()"
except TypeError:
    pass

try:
    s = {[1, 2]}
    assert False, "expected TypeError for list in set literal"
except TypeError:
    pass

try:
    s = set()
    s.add([1, 2])
    assert False, "expected TypeError for set.add(list)"
except TypeError:
    pass

try:
    s = {x for x in [[1], [2]]}
    assert False, "expected TypeError for list in set comprehension"
except TypeError:
    pass

# Hashable types must still work
s = set([1, 1.0, True])   # 1, 1.0, True all hash to same key
assert len(s) == 1

s2 = set([(1, 2), (3, 4), (1, 2)])
assert len(s2) == 2

print("Set hashability tests passed!")
