# Test map() builtin

# Basic map with lambda
numbers = [1, 2, 3, 4, 5]
squared = list(map(lambda x: x * x, numbers))
assert squared == [1, 4, 9, 16, 25]

# Map with function
def double(x):
    return x * 2

doubled = list(map(double, numbers))
assert doubled == [2, 4, 6, 8, 10]

# Map with builtin function
strings = ["hello", "world"]
upper_strings = list(map(str.upper, strings))
assert upper_strings == ["HELLO", "WORLD"]

# Map with multiple iterables
list1 = [1, 2, 3]
list2 = [10, 20, 30]
sums = list(map(lambda x, y: x + y, list1, list2))
assert sums == [11, 22, 33]

# Map with string
chars = list(map(lambda c: c.upper(), "hello"))
assert chars == ["H", "E", "L", "L", "O"]

# Map returns iterator
result = map(lambda x: x + 1, [1, 2, 3])
assert list(result) == [2, 3, 4]

# Empty map
empty = list(map(lambda x: x, []))
assert empty == []

print("All map() tests passed!")
