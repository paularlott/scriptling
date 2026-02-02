# Test sorted() with reverse parameter

# Basic sorted with reverse
nums = [3, 1, 4, 1, 5, 9, 2, 6]
s1 = sorted(nums)
assert s1[0] == 1 and s1[1] == 1 and s1[7] == 9 and len(s1) == 8
s2 = sorted(nums, reverse=True)
assert s2[0] == 9 and s2[1] == 6 and s2[7] == 1 and len(s2) == 8

# Sorted strings
words = ["banana", "apple", "cherry", "date"]
s3 = sorted(words)
assert s3[0] == 'apple' and s3[3] == 'date'
s4 = sorted(words, reverse=True)
assert s4[0] == 'date' and s4[3] == 'apple'

# Sorted with explicit False
s5 = sorted([5, 2, 8, 1], reverse=False)
assert s5[0] == 1 and s5[3] == 8

# Empty list
s6 = sorted([], reverse=True)
assert len(s6) == 0

# Single element
s7 = sorted([42], reverse=True)
assert len(s7) == 1 and s7[0] == 42
