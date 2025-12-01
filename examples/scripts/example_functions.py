# Example: Functions
# Function definition, parameters, return values, recursion

print("Functions")

# Simple function
def greet(name):
    return "Hello, " + name

result = greet("World")
print(f"greet('World'): {result}")

# Multiple parameters
def add(a, b):
    return a + b

result = add(10, 20)
print(f"add(10, 20): {result}")

# Function with no return (implicit None)
def print_message(msg):
    print(f"Message: {msg}")

print_message("Test")

# Recursive function
def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)

result = factorial(5)
print(f"factorial(5): {result}")

# Nested function calls
def double(x):
    return x * 2

def square(x):
    return x * x

result = double(square(3))
print(f"double(square(3)): {result}")

