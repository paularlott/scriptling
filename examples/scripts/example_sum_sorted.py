#!/usr/bin/env scriptling

# Test script for sum() and sorted() builtins

print("Testing sum() builtin...")

# Test 1: Basic sum with integers
print("\nTest 1: Basic sum with integers")
print("sum([1, 2, 3, 4, 5]) =", sum([1, 2, 3, 4, 5]), "(expected 15)")
print("sum([10, 20, 30]) =", sum([10, 20, 30]), "(expected 60)")

# Test 2: Sum with empty list
print("\nTest 2: Sum with empty list")
print("sum([]) =", sum([]), "(expected 0)")

# Test 3: Sum with single element
print("\nTest 3: Sum with single element")
print("sum([42]) =", sum([42]), "(expected 42)")

# Test 4: Sum with floats
print("\nTest 4: Sum with floats")
result = sum([1.5, 2.5, 3.0])
print("sum([1.5, 2.5, 3.0]) =", result, "(expected 7.0)")

# Test 5: Sum with mixed int and float
print("\nTest 5: Sum with mixed int and float")
result = sum([1, 2.5, 3, 1.5])
print("sum([1, 2.5, 3, 1.5]) =", result, "(expected 8.0)")

# Test 6: Sum with negative numbers
print("\nTest 6: Sum with negative numbers")
print("sum([-1, -2, -3]) =", sum([-1, -2, -3]), "(expected -6)")
print("sum([10, -5, 3, -8]) =", sum([10, -5, 3, -8]), "(expected 0)")

# Test 7: Sum with tuples
print("\nTest 7: Sum with tuples")
print("sum((1, 2, 3, 4)) =", sum((1, 2, 3, 4)), "(expected 10)")

print("\nTesting sorted() builtin...")

# Test 8: Basic sorted with integers
print("\nTest 8: Basic sorted with integers")
result = sorted([3, 1, 4, 1, 5, 9, 2, 6])
print("sorted([3, 1, 4, 1, 5, 9, 2, 6]) =", result)
print("(expected [1, 1, 2, 3, 4, 5, 6, 9])")

# Test 9: Sorted with strings
print("\nTest 9: Sorted with strings")
result = sorted(["banana", "apple", "cherry"])
print("sorted(['banana', 'apple', 'cherry']) =", result)
print("(expected ['apple', 'banana', 'cherry'])")

# Test 10: Sorted with already sorted list
print("\nTest 10: Sorted with already sorted list")
result = sorted([1, 2, 3, 4, 5])
print("sorted([1, 2, 3, 4, 5]) =", result, "(expected [1, 2, 3, 4, 5])")

# Test 11: Sorted with reverse sorted list
print("\nTest 11: Sorted with reverse sorted list")
result = sorted([5, 4, 3, 2, 1])
print("sorted([5, 4, 3, 2, 1]) =", result, "(expected [1, 2, 3, 4, 5])")

# Test 12: Sorted doesn't modify original
print("\nTest 12: Sorted doesn't modify original")
original = [3, 1, 2]
result = sorted(original)
print("sorted([3, 1, 2]) =", result, "(expected [1, 2, 3])")
print("original still:", original, "(expected [3, 1, 2])")

# Test 13: Sorted with floats
print("\nTest 13: Sorted with floats")
result = sorted([3.14, 1.41, 2.71, 0.5])
print("sorted([3.14, 1.41, 2.71, 0.5]) =", result)
print("(expected [0.5, 1.41, 2.71, 3.14])")

# Test 14: Sorted with mixed int and float
print("\nTest 14: Sorted with mixed int and float")
result = sorted([3, 1.5, 2, 0.5, 4])
print("sorted([3, 1.5, 2, 0.5, 4]) =", result)
print("(expected [0.5, 1.5, 2, 3, 4])")

# Test 15: Sorted with negative numbers
print("\nTest 15: Sorted with negative numbers")
result = sorted([3, -1, 0, -5, 2])
print("sorted([3, -1, 0, -5, 2]) =", result, "(expected [-5, -1, 0, 2, 3])")

# Test 16: Sorted with single element
print("\nTest 16: Sorted with single element")
result = sorted([42])
print("sorted([42]) =", result, "(expected [42])")

# Test 17: Sorted with empty list
print("\nTest 17: Sorted with empty list")
result = sorted([])
print("sorted([]) =", result, "(expected [])")

# Test 18: Sorted with tuple
print("\nTest 18: Sorted with tuple")
result = sorted((3, 1, 4, 1, 5))
print("sorted((3, 1, 4, 1, 5)) =", result, "(expected [1, 1, 3, 4, 5])")

# Test 19: Sorted with key=len for strings
print("\nTest 19: Sorted with key=len for strings")
words = ["banana", "pie", "Washington", "car"]
result = sorted(words, len)
print("sorted(['banana', 'pie', 'Washington', 'car'], len) =", result)
print("(expected ['pie', 'car', 'banana', 'Washington'])")

# Test 20: Combined sum and sorted
print("\nTest 20: Combined sum and sorted")
numbers = [5, 2, 8, 1, 9]
sorted_numbers = sorted(numbers)
total = sum(sorted_numbers)
print("sorted([5, 2, 8, 1, 9]) =", sorted_numbers, "(expected [1, 2, 5, 8, 9])")
print("sum(sorted_numbers) =", total, "(expected 25)")

# Test 21: Sum of sorted list
print("\nTest 21: Sum of sorted list")
result = sum(sorted([5, 1, 3, 2, 4]))
print("sum(sorted([5, 1, 3, 2, 4])) =", result, "(expected 15)")

# Test 22: Sorted in list comprehension
print("\nTest 22: Sorted in list comprehension")
numbers = [[3, 1, 2], [6, 4, 5], [9, 7, 8]]
sorted_lists = [sorted(nums) for nums in numbers]
print("Results:", sorted_lists)
print("(expected [[1, 2, 3], [4, 5, 6], [7, 8, 9]])")

print("\n✓ All sum() and sorted() tests completed!")

