# Basic tuple access
coords = (10, 20, 30)
assert coords[0] == 10
assert coords[1] == 20
assert coords[2] == 30
assert coords[-1] == 30
assert len(coords) == 3

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

# Single-element tuple
single = (42,)
assert len(single) == 1
assert single[0] == 42

# Empty tuple
empty = ()
assert len(empty) == 0

# Tuple in / not in (convert to list first)
assert 20 in list(coords)
assert 99 not in list(coords)

# Tuple iteration
total = 0
for v in coords:
    total += v
assert total == 60

# Tuple as dict key
d = {}
d[(1, 2)] = "point"
assert d[(1, 2)] == "point"

# Nested tuples
matrix = ((1, 2), (3, 4))
assert matrix[0][1] == 2
assert matrix[1][0] == 3

# tuple() constructor
t = tuple([1, 2, 3])
assert t == (1, 2, 3)
