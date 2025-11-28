# Test sorted() with reverse parameter

# Basic sorted with reverse
nums = [3, 1, 4, 1, 5, 9, 2, 6]
s1 = sorted(nums)
r1 = s1[0] == 1 and s1[1] == 1 and s1[7] == 9 and len(s1) == 8
s2 = sorted(nums, reverse=True)
r2 = s2[0] == 9 and s2[1] == 6 and s2[7] == 1 and len(s2) == 8

# Sorted strings
words = ["banana", "apple", "cherry", "date"]
s3 = sorted(words)
r3 = s3[0] == 'apple' and s3[3] == 'date'
s4 = sorted(words, reverse=True)
r4 = s4[0] == 'date' and s4[3] == 'apple'

# Sorted with explicit False
s5 = sorted([5, 2, 8, 1], reverse=False)
r5 = s5[0] == 1 and s5[3] == 8

# Empty list
s6 = sorted([], reverse=True)
r6 = len(s6) == 0

# Single element
s7 = sorted([42], reverse=True)
r7 = len(s7) == 1 and s7[0] == 42

r1 and r2 and r3 and r4 and r5 and r6 and r7
