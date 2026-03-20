# Implicit tuple packing in assignment
t = 1, 2, 3
assert t == (1, 2, 3)
assert type(t) == "TUPLE"

# Single element with trailing comma (bare)
s = 42,
assert s == (42,)
assert len(s) == 1
assert s[0] == 42

# Single element with trailing comma (parenthesised) — the main case
t = (42,)
assert type(t) == "TUPLE"
assert len(t) == 1
assert t[0] == 42
assert t == (42,)

# Empty tuple
e = ()
assert type(e) == "TUPLE"
assert len(e) == 0

# Multi-element parenthesised
t2 = (1, 2, 3)
assert t2 == (1, 2, 3)
assert len(t2) == 3

# Nested
t3 = ((1, 2), (3, 4))
assert t3[0] == (1, 2)
assert t3[1][1] == 4

# Tuple concatenation
a = (1,) + (2, 3)
assert a == (1, 2, 3)
assert type(a) == "TUPLE"

b = (1, 2) + (3,)
assert b == (1, 2, 3)

c = () + (1,)
assert c == (1,)

# Tuple repetition
r = (1, 2) * 3
assert r == (1, 2, 1, 2, 1, 2)
assert len(r) == 6

r2 = 3 * (1, 2)
assert r2 == (1, 2, 1, 2, 1, 2)

r3 = (1,) * 0
assert r3 == ()

# Tuple equality / inequality
assert (1,) == (1,)
assert (1,) != (2,)
assert (1, 2) != (1,)

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

# Single-element tuple in function call
def f(x): return x
assert f((42,)) == (42,)

# Single-element tuple in list
lst = [(1,), (2,), (3,)]
assert lst[0] == (1,)

# Single-element tuple as dict key
d = {(1,): "a", (2,): "b"}
assert d[(1,)] == "a"

# Single-element tuple in set
s2 = {(1,), (2,), (1,)}
assert len(s2) == 2

print("Tuple tests passed!")
