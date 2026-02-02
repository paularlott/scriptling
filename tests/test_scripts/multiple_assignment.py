# Two variables
x, y = [10, 20]
assert x == 10
assert y == 20

# Three variables
a, b, c = [1, 2, 3]
assert a == 1
assert b == 2
assert c == 3

# Swap
x, y = [y, x]
assert x == 20
assert y == 10

# From function
def get_coords():
    return [100, 200]

x, y = get_coords()
assert x == 100
assert y == 200

# Mixed types
name, age = ["Alice", 30]
assert name == "Alice"
assert age == 30