# Example: Generator Expressions
# Generator expressions are like list comprehensions but with parentheses

print("Generator Expressions")

# Test 1: Generator in function call (no extra parens needed)
text = "this is a test string"
result = ' '.join(word.upper() for word in text.split())
print(f"Uppercase words: {result}")

# Test 2: Generator with parentheses
numbers = [1, 2, 3, 4, 5]
doubled = (x * 2 for x in numbers)
print(f"Doubled: {doubled}")

# Test 3: Generator with condition
evens = (x for x in numbers if x % 2 == 0)
print(f"Even numbers: {evens}")

# Test 4: Multiple generators in one expression
words = ["hello", "world"]
result2 = ', '.join(w.upper() for w in words)
print(f"Joined: {result2}")

