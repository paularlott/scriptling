# Test filter() builtin

# Basic filter with lambda
numbers = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
evens = list(filter(lambda x: x % 2 == 0, numbers))
assert evens == [2, 4, 6, 8, 10]

# Filter with function
def is_positive(x):
    return x > 0

mixed = [-2, -1, 0, 1, 2, 3]
positives = list(filter(is_positive, mixed))
assert positives == [1, 2, 3]

# Filter with None (filters out falsy values)
values = [0, 1, False, True, "", "hello", None, [], [1, 2]]
truthy = list(filter(None, values))
assert truthy == [1, True, "hello", [1, 2]]

# Filter strings
words = ["apple", "banana", "cherry", "date"]
long_words = list(filter(lambda w: len(w) > 5, words))
assert long_words == ["banana", "cherry"]

# Filter returns iterator
result = filter(lambda x: x > 5, [1, 6, 3, 8, 2, 9])
assert list(result) == [6, 8, 9]

# Empty filter
empty = list(filter(lambda x: x > 10, [1, 2, 3]))
assert empty == []

# Filter all elements
all_pass = list(filter(lambda x: True, [1, 2, 3]))
assert all_pass == [1, 2, 3]

print("All filter() tests passed!")
