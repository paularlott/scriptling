failures = 0

# Two variables
x, y = [10, 20]
if x != 10 or y != 20:
    failures += 1

# Three variables
a, b, c = [1, 2, 3]
if a != 1 or b != 2 or c != 3:
    failures += 1

# Swap
x, y = [y, x]
if x != 20 or y != 10:
    failures += 1

# From function
def get_coords():
    return [100, 200]

x, y = get_coords()
if x != 100 or y != 200:
    failures += 1

# Mixed types
name, age = ["Alice", 30]
if name != "Alice" or age != 30:
    failures += 1

failures == 0