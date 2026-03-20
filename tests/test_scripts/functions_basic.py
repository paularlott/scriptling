# Basic function
def greet(name):
    return "Hello, " + name

assert greet("World") == "Hello, World"

# Multiple parameters
def add(a, b):
    return a + b

assert add(3, 4) == 7

# Default parameters
def power(base, exp=2):
    return base ** exp

assert power(3) == 9
assert power(3, 3) == 27

# Return multiple values (tuple packing)
def min_max(lst):
    return min(lst), max(lst)

lo, hi = min_max([3, 1, 4, 1, 5, 9])
assert lo == 1
assert hi == 9

# Function as value
def double(x):
    return x * 2

fn = double
assert fn(5) == 10

# Function called in expression
assert add(power(2), power(3)) == 4 + 9

# Early return
def first_positive(lst):
    for x in lst:
        if x > 0:
            return x
    return None

assert first_positive([-1, -2, 3, 4]) == 3
assert first_positive([-1, -2]) == None

# Recursive default (no mutable default trap — use None sentinel)
def append_to(val, lst=None):
    if lst is None:
        lst = []
    lst.append(val)
    return lst

assert append_to(1) == [1]
assert append_to(2) == [2]  # fresh list each time
