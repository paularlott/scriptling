# Test iterator behavior (Python 3 compatibility)

# Test that range() returns an iterator that works in for loops
count = 0
for i in range(5):
    count += 1
assert count == 5, "range() should work in for loops"

# Test range with start, stop
total = 0
for i in range(2, 7):
    total += i
assert total == 20, "range(2, 7) should produce 2,3,4,5,6"

# Test range with step
result = []
for i in range(0, 10, 2):
    result.append(i)
assert result == [0, 2, 4, 6, 8], "range with step should work"

# Test negative step
result = []
for i in range(10, 0, -2):
    result.append(i)
assert result == [10, 8, 6, 4, 2], "range with negative step should work"

# Test list(range()) conversion
nums = list(range(5))
assert nums == [0, 1, 2, 3, 4], "list(range()) should convert iterator to list"

# Test enumerate() returns an iterator
result = []
for i, v in enumerate(["a", "b", "c"]):
    result.append((i, v))
assert result == [(0, "a"), (1, "b"), (2, "c")], "enumerate should work in for loops"

# Test enumerate with start parameter
result = []
for i, v in enumerate(["x", "y"], 10):
    result.append((i, v))
assert result == [(10, "x"), (11, "y")], "enumerate with start should work"

# Test list(enumerate())
enum_list = list(enumerate(["a", "b"]))
assert len(enum_list) == 2
assert enum_list[0] == (0, "a")
assert enum_list[1] == (1, "b")

# Test zip() returns an iterator
result = []
for x, y in zip([1, 2, 3], ["a", "b", "c"]):
    result.append((x, y))
assert result == [(1, "a"), (2, "b"), (3, "c")], "zip should work in for loops"

# Test zip with different lengths (stops at shortest)
result = []
for x, y in zip([1, 2], ["a", "b", "c", "d"]):
    result.append((x, y))
assert result == [(1, "a"), (2, "b")], "zip should stop at shortest iterable"

# Test list(zip())
zipped = list(zip([1, 2], ["a", "b"]))
assert len(zipped) == 2
assert zipped[0] == (1, "a")

# Test reversed() returns an iterator
result = []
for x in reversed([1, 2, 3]):
    result.append(x)
assert result == [3, 2, 1], "reversed should work in for loops"

# Test list(reversed())
rev = list(reversed([1, 2, 3]))
assert rev == [3, 2, 1]

# Test reversed with string
result = []
for ch in reversed("abc"):
    result.append(ch)
assert result == ["c", "b", "a"], "reversed should work with strings"

# Test iterator consumption - iterators can only be consumed once
r = range(3)
first_pass = []
for i in r:
    first_pass.append(i)
assert first_pass == [0, 1, 2], "First iteration should work"

# After consumption, a second iteration should produce no elements
second_pass = []
for i in r:
    second_pass.append(i)
assert second_pass == [], "Second iteration should be empty (iterator exhausted)"

# Test list comprehension with iterator
squares = [x * x for x in range(5)]
assert squares == [0, 1, 4, 9, 16], "List comprehension should work with range iterator"

# Test nested iterators
result = []
for i in range(2):
    for j in range(2):
        result.append((i, j))
assert result == [(0, 0), (0, 1), (1, 0), (1, 1)], "Nested for loops with iterators should work"

# Test map returns iterator
doubled = list(map(lambda x: x * 2, [1, 2, 3]))
assert doubled == [2, 4, 6], "map should work"

# Test map works in for loop
result = []
for x in map(lambda x: x * 2, [1, 2, 3]):
    result.append(x)
assert result == [2, 4, 6], "map iterator should work in for loop"

# Test filter returns iterator
evens = list(filter(lambda x: x % 2 == 0, [1, 2, 3, 4, 5]))
assert evens == [2, 4], "filter should work"

# Test filter works in for loop
result = []
for x in filter(lambda x: x > 2, [1, 2, 3, 4, 5]):
    result.append(x)
assert result == [3, 4, 5], "filter iterator should work in for loop"

# Test filter with None (truthy filter)
result = list(filter(None, [0, 1, "", "hello", [], [1, 2]]))
assert result == [1, "hello", [1, 2]], "filter with None should filter falsy values"
print("All iterator tests passed!")
