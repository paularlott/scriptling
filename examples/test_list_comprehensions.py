# List Comprehensions Test

# Basic list comprehension
squares = [x * x for x in range(5)]
print("Squares:", squares)

# List comprehension with condition
evens = [x for x in range(10) if x % 2 == 0]
print("Evens:", evens)

# List comprehension with string
chars = [c for c in "hello"]
print("Chars:", chars)

# List comprehension with transformation
doubled = [x * 2 for x in [1, 2, 3, 4, 5]]
print("Doubled:", doubled)

# List comprehension with condition and transformation
filtered_squares = [x * x for x in range(10) if x > 5]
print("Filtered squares:", filtered_squares)

# Simple list from existing list
numbers = [1, 2, 3, 4, 5]
incremented = [x + 1 for x in numbers]
print("Incremented:", incremented)

print("List comprehensions test completed")