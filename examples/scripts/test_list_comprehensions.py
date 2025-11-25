# Test: List comprehensions

print("=== Testing List Comprehensions ===")

# Basic comprehension
numbers = [i for i in range(5)]
print(f"Basic [i for i in range(5)]: {numbers}")

# With expression
squares = [i * i for i in range(5)]
print(f"Squares [i*i for i in range(5)]: {squares}")

# With condition
evens = [i for i in range(10) if i % 2 == 0]
print(f"Evens [i for i in range(10) if i%2==0]: {evens}")

# From list
original = [1, 2, 3, 4, 5]
doubled = [x * 2 for x in original]
print(f"Doubled: {doubled}")

# String comprehension
text = "hello"
chars = [c for c in text]
print(f"Chars from 'hello': {chars}")

# Nested list comprehension
matrix = [[i * j for j in range(3)] for i in range(3)]
print(f"Matrix: {matrix}")

# Complex expression
results = [i * 2 + 1 for i in range(5)]
print(f"Complex [i*2+1 for i in range(5)]: {results}")

print("✓ All list comprehension tests passed")
