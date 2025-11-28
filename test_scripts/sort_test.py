# Test sort functionality

# Basic sort
nums = [5, 2, 8, 1, 9, 3]
nums.sort()
assert nums == [1, 2, 3, 5, 8, 9]

# Reverse sort
nums = [5, 2, 8, 1, 9, 3]
nums.sort(reverse=True)
assert nums == [9, 8, 5, 3, 2, 1]

# Sort with key function
words = ["banana", "pie", "apple", "cherry"]
words.sort(key=len)
assert words == ["pie", "apple", "banana", "cherry"]

# Sort with key and reverse
words = ["banana", "pie", "apple", "cherry"]
words.sort(key=len, reverse=True)
assert words == ["banana", "cherry", "apple", "pie"]

# Test sorted() builtin
nums = [5, 2, 8, 1, 9, 3]
result = sorted(nums)
assert result == [1, 2, 3, 5, 8, 9]
assert nums == [5, 2, 8, 1, 9, 3]  # Original unchanged

# Test sorted with reverse
result = sorted([5, 2, 8, 1, 9, 3], reverse=True)
assert result == [9, 8, 5, 3, 2, 1]

# Test sorted with key
result = sorted(["banana", "pie", "apple"], len)
assert result == ["pie", "apple", "banana"]

# Test string sort
words = ["banana", "apple", "cherry"]
words.sort()
assert words == ["apple", "banana", "cherry"]

# Test mixed int/float sort
nums = [3.5, 1, 2.5, 4, 1.5]
nums.sort()
assert nums == [1, 1.5, 2.5, 3.5, 4]

True
