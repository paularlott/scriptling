# Test: Math library

import math

print("=== Testing Math Library ===")

# sqrt
result = math.sqrt(16)
print(f"sqrt(16) = {result}")

# pow
result = math.pow(2, 8)
print(f"pow(2, 8) = {result}")

# abs
result = math.abs(-42)
print(f"abs(-42) = {result}")

# floor
result = math.floor(3.7)
print(f"floor(3.7) = {result}")

# ceil
result = math.ceil(3.2)
print(f"ceil(3.2) = {result}")

# round
result = math.round(3.5)
print(f"round(3.5) = {result}")

# min with multiple args
result = math.min(3, 1, 4, 1, 5)
print(f"min(3, 1, 4, 1, 5) = {result}")

# max with multiple args
result = math.max(3, 1, 4, 1, 5)
print(f"max(3, 1, 4, 1, 5) = {result}")

# Constants
pi = math.pi()
print(f"pi = {pi}")

e = math.e()
print(f"e = {e}")

# Calculate circle area
radius = 5
area = math.pi() * math.pow(radius, 2)
print(f"Circle area (r=5): {area}")

print("✓ All math library tests passed")
