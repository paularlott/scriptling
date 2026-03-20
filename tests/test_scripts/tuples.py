# Basic access and length
coords = (10, 20, 30)
assert coords[0] == 10
assert coords[1] == 20
assert coords[2] == 30
assert coords[-1] == 30
assert coords[-2] == 20
assert len(coords) == 3

# Single-element tuple — trailing comma required
single = (42,)
assert len(single) == 1
assert single[0] == 42
assert (42) == 42       # no trailing comma = just grouping

# Empty tuple
empty = ()
assert len(empty) == 0

# Implicit packing — parentheses optional
t = 1, 2, 3
assert t == (1, 2, 3)
s = 42,
assert s == (42,)

# Tuple unpacking
x, y, z = coords
assert x == 10
assert y == 20
assert z == 30

# Swap via tuple unpacking
p, q = 1, 2
p, q = q, p
assert p == 2
assert q == 1

# Extended unpacking
first, *rest = (1, 2, 3, 4)
assert first == 1
assert rest == [2, 3, 4]

*init, last = (1, 2, 3, 4)
assert init == [1, 2, 3]
assert last == 4

# Nested tuples
matrix = ((1, 2), (3, 4))
assert matrix[0][1] == 2
assert matrix[1][0] == 3

# Slicing returns a tuple
assert coords[1:3] == (20, 30)
assert coords[:2] == (10, 20)
assert coords[1:] == (20, 30)
assert coords[::2] == (10, 30)
assert coords[::-1] == (30, 20, 10)

# in / not in
assert 20 in coords
assert 99 not in coords
assert 10 in coords
assert 30 in coords

# Iteration
total = 0
for v in coords:
    total += v
assert total == 60

# Concatenation and repetition
assert (1, 2) + (3, 4) == (1, 2, 3, 4)
assert (1, 2) * 3 == (1, 2, 1, 2, 1, 2)
assert 3 * (1, 2) == (1, 2, 1, 2, 1, 2)
assert () + (1,) == (1,)

# count() and index()
t = (1, 2, 2, 3, 2)
assert t.count(2) == 3
assert t.count(1) == 1
assert t.count(9) == 0
assert t.index(1) == 0
assert t.index(2) == 1
assert t.index(3) == 3

# index() with start/end bounds
assert t.index(2, 2) == 2    # search from index 2
assert t.index(2, 3) == 4    # search from index 3

# tuple() constructor
assert tuple([1, 2, 3]) == (1, 2, 3)
assert tuple("ab") == ("a", "b")
assert tuple(range(3)) == (0, 1, 2)
assert tuple(()) == ()

# Hashable — usable as dict key and set element
d = {}
d[(1, 2)] = "point"
assert d[(1, 2)] == "point"

point_set = {(1, 2), (3, 4), (1, 2)}
assert len(point_set) == 2

# Immutability — assignment to index raises error
caught = False
try:
    coords[0] = 99
except:
    caught = True
assert caught

# Multiple return values use implicit tuple packing
def min_max(lst):
    return min(lst), max(lst)

lo, hi = min_max([3, 1, 4, 1, 5, 9])
assert lo == 1
assert hi == 9

# Tuple in list comprehension
pairs = [(x, y) for x in range(3) for y in range(2)]
assert len(pairs) == 6
assert pairs[0] == (0, 0)
assert pairs[5] == (2, 1)

print("Tuple tests passed!")
