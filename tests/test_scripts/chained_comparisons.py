# Basic chained comparisons
x = 5
assert 1 < x < 10
assert 0 < x <= 5
assert 5 <= x <= 5
assert 1 < x < 10 < 20

# Chained equality
assert 5 == 5 == 5
a = 5
b = 5
assert a == b == 5

# Chained with variables
lo, hi = 0, 100
val = 42
assert lo < val < hi
assert lo <= val <= hi

# False chained comparisons
assert not (10 < x < 20)
assert not (0 < x < 3)

# Chained in function
def in_range(n, low, high):
    return low <= n <= high

assert in_range(5, 1, 10)
assert in_range(1, 1, 10)
assert in_range(10, 1, 10)
assert not in_range(0, 1, 10)
assert not in_range(11, 1, 10)

# Chained with expressions
assert 1 < 2 + 1 < 10
assert 0 < len("hello") < 10
