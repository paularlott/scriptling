# Test: Lambda functions

print("=== Testing Lambda Functions ===")

# Basic lambda
double = lambda x: x * 2
result = double(5)
print(f"lambda x: x * 2, applied to 5: {result}")

# Lambda with multiple parameters
add = lambda a, b: a + b
result = add(3, 7)
print(f"lambda a, b: a + b, applied to 3, 7: {result}")

# Lambda in variable
multiply = lambda x, y: x * y
result = multiply(4, 5)
print(f"lambda x, y: x * y, applied to 4, 5: {result}")

# Lambda with list operations
numbers = [1, 2, 3, 4, 5]
square = lambda x: x * x
squares = [square(n) for n in numbers]
print(f"Squares using lambda: {squares}")

# Lambda for comparison
is_positive = lambda x: x > 0
print(f"is_positive(5): {is_positive(5)}")
print(f"is_positive(-3): {is_positive(-3)}")

# Lambda returning boolean
is_even = lambda n: n % 2 == 0
print(f"is_even(4): {is_even(4)}")
print(f"is_even(7): {is_even(7)}")

print("✓ All lambda function tests passed")
