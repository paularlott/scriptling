# Example: Using the math library in Scriptling
# Demonstrates mathematical functions and constants

import math

print("Math Library Functions and Constants")

# Basic mathematical operations
result = math.sqrt(16)
print(f"sqrt(16) = {result}")

result = math.pow(2, 8)
print(f"pow(2, 8) = {result}")

# abs is a builtin, not in math module (Python 3 compatible)
result = abs(-42)
print(f"abs(-42) = {result}")

# math.fabs returns float absolute value
result = math.fabs(-42.5)
print(f"fabs(-42.5) = {result}")

# Rounding functions
result = math.floor(3.7)
print(f"floor(3.7) = {result}")

result = math.ceil(3.2)
print(f"ceil(3.2) = {result}")

result = round(3.5)  # Note: round is builtin, not math.round
print(f"round(3.5) = {result}")

# min and max are builtins (Python 3 compatible)
result = min(3, 1, 4, 1, 5)
print(f"min(3, 1, 4, 1, 5) = {result}")

result = max(3, 1, 4, 1, 5)
print(f"max(3, 1, 4, 1, 5) = {result}")

# Trigonometric functions
result = math.sin(0)
print(f"sin(0) = {result}")

result = math.cos(0)
print(f"cos(0) = {result}")

result = math.tan(0)
print(f"tan(0) = {result}")

# Logarithmic and exponential functions
result = math.log(1)
print(f"log(1) = {result}")

result = math.exp(0)
print(f"exp(0) = {result}")

# Angle conversion
result = math.degrees(math.pi)
print(f"degrees(π) = {result}")

result = math.radians(180)
print(f"radians(180) = {result}")

# Modular arithmetic
result = math.fmod(5.5, 2.0)
print(f"fmod(5.5, 2.0) = {result}")

result = math.gcd(48, 18)
print(f"gcd(48, 18) = {result}")

result = math.factorial(5)
print(f"factorial(5) = {result}")

# Mathematical constants
pi = math.pi
print(f"π = {pi}")

e = math.e
print(f"e = {e}")

# Practical example: Calculate circle area
radius = 5
area = math.pi * math.pow(radius, 2)
print(f"Circle area with radius {radius}: {area}")
