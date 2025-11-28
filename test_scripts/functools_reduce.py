# Test functools library
import functools

# Test reduce with a function
def add(x, y):
    return x + y

result = functools.reduce(add, [1, 2, 3, 4, 5])
result == 15

# Test reduce with initial value
result = functools.reduce(add, [1, 2, 3], 10)
result == 16

# Test reduce with multiply
def multiply(x, y):
    return x * y

result = functools.reduce(multiply, [1, 2, 3, 4])
result == 24

# Test reduce with single element
result = functools.reduce(add, [42])
result == 42

# Test reduce to find max
def max_fn(a, b):
    if a > b:
        return a
    return b

result = functools.reduce(max_fn, [3, 1, 4, 1, 5, 9, 2, 6])
result == 9

# Test reduce to build a string
def concat(a, b):
    return str(a) + str(b)

result = functools.reduce(concat, ["a", "b", "c"])
result == "abc"
