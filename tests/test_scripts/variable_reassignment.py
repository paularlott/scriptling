# Basic reassignment
x = 5
x = 10
assert x == 10

# Type change on reassignment
x = "hello"
assert x == "hello"
x = [1, 2, 3]
assert len(x) == 3

# Chained assignment
a = b = c = 42
assert a == 42
assert b == 42
assert c == 42

# Tuple packing assignment
t = 1, 2, 3
assert t == (1, 2, 3)

# Swap via tuple unpacking
p, q = 10, 20
p, q = q, p
assert p == 20
assert q == 10

# Augmented assignments
n = 10
n += 5
assert n == 15
n -= 3
assert n == 12
n *= 2
assert n == 24
n //= 4
assert n == 6
n **= 2
assert n == 36
n %= 10
assert n == 6

# Multiple assignment (unpacking)
a, b, c = [1, 2, 3]
assert a == 1
assert b == 2
assert c == 3

# Starred unpacking
first, *rest = [1, 2, 3, 4, 5]
assert first == 1
assert rest == [2, 3, 4, 5]

*init, last = [1, 2, 3, 4, 5]
assert init == [1, 2, 3, 4]
assert last == 5
