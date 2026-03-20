# Implicit tuple packing in assignment
t = 1, 2, 3
assert t == (1, 2, 3)
assert type(t) == "TUPLE"

# Single element with trailing comma
s = 42,
assert s == (42,)
assert len(s) == 1

# Return multiple values
def min_max(lst):
    return min(lst), max(lst)

lo, hi = min_max([3, 1, 4, 1, 5, 9, 2, 6])
assert lo == 1
assert hi == 9

# Swap
a, b = 1, 2
a, b = b, a
assert a == 2
assert b == 1

# Chained assignment
x = y = z = 99
assert x == 99
assert y == 99
assert z == 99

# Tuple packing in return used as tuple
def coords():
    return 10, 20

c = coords()
assert c[0] == 10
assert c[1] == 20

# Nested unpacking via intermediate
def stats(lst):
    return min(lst), max(lst), sum(lst)

result = stats([1, 2, 3, 4, 5])
assert result[0] == 1
assert result[1] == 5
assert result[2] == 15

mn, mx, total = stats([1, 2, 3, 4, 5])
assert mn == 1
assert mx == 5
assert total == 15
