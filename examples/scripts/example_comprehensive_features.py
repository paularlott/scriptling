#!/usr/bin/env scriptling

# Comprehensive Python Features Test for Scriptling

print("Comprehensive Python Features ===\n")

# Test 1: List comprehensions with power operator
print("Test 1: List comprehensions with power operator")
numbers = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
even_squares = [x**2 for x in numbers if x % 2 == 0]
print("Even squares:", even_squares)
print("Expected: [4, 16, 36, 64, 100]\n")

# Test 2: Dictionary operations with sum()
print("Test 2: Dictionary operations with sum()")
student_grades = {"Alice": 85, "Bob": 92, "Charlie": 78}
grade_values = list(student_grades.values())
average = sum(grade_values) / len(student_grades)
print("Grades:", grade_values)
print("Average:", average)
print("Expected: 85.0\n")

# Test 3: sorted() with key function
print("Test 3: sorted() with key function")
words = ["banana", "pie", "Washington", "car"]
sorted_words = sorted(words, key=len)
print("Words sorted by length:", sorted_words)
print("Expected: ['pie', 'car', 'banana', 'Washington']\n")

# Test 4: Multiple assignment with expressions
print("Test 4: Multiple assignment with expressions")
a, b = 1, 2
print(f"a = {a}, b = {b}")
a, b = b, a + b
print(f"After swap: a = {a}, b = {b}")
print("Expected: a = 2, b = 3\n")

# Test 5: Generator expressions (evaluated as list comprehensions)
print("Test 5: Generator expressions")
text = "hello world from scriptling"
title_text = " ".join(word.capitalize() for word in text.split())
print("Title case:", title_text)
print("Expected: Hello World From Scriptling\n")

# Test 6: Nested list comprehensions with power
print("Test 6: Nested list comprehensions with power")
matrix = [[i**2 for i in range(3)] for j in range(3)]
print("Matrix:", matrix)
print("Expected: [[0, 1, 4], [0, 1, 4], [0, 1, 4]]\n")

# Test 7: Combined operations
print("Test 7: Combined operations")
data = [3, 1, 4, 1, 5, 9, 2, 6]
sorted_data = sorted(data)
total = sum(sorted_data)
avg = total / len(sorted_data)
print("Sorted:", sorted_data)
print("Sum:", total)
print("Average:", avg)
print("Expected: sorted=[1, 1, 2, 3, 4, 5, 6, 9], sum=31, avg=3.875\n")

# Test 8: String methods with list comprehensions
print("Test 8: String methods with list comprehensions")
names = ["alice", "bob", "charlie"]
capitalized = [name.capitalize() for name in names]
print("Capitalized:", capitalized)
print("Expected: ['Alice', 'Bob', 'Charlie']\n")

# Test 9: Power operator precedence
print("Test 9: Power operator precedence")
result1 = 2 + 3**2
result2 = (2 + 3)**2
result3 = 2 * 3**2
print("2 + 3**2 =", result1, "(expected 11)")
print("(2 + 3)**2 =", result2, "(expected 25)")
print("2 * 3**2 =", result3, "(expected 18)\n")

# Test 10: All features combined
print("Test 10: All features combined")
raw_scores = [85, 92, 78, 95, 88]
normalized = [(score - sum(raw_scores)/len(raw_scores))**2 for score in raw_scores]
variance = sum(normalized) / len(normalized)
print("Raw scores:", raw_scores)
print("Variance:", variance)
print("Expected variance: ~42.56\n")

print("=== All comprehensive tests completed!")
