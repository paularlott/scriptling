# Test: Basic Python features
# Variables, arithmetic, comparisons, print

print("=== Testing Basic Features ===")

# Variables and arithmetic
x = 10
y = 5
print(f"x = {x}, y = {y}")

sum_val = x + y
print(f"Addition: {x} + {y} = {sum_val}")

diff = x - y
print(f"Subtraction: {x} - {y} = {diff}")

prod = x * y
print(f"Multiplication: {x} * {y} = {prod}")

quot = x / y
print(f"Division: {x} / {y} = {quot}")

mod = x % y
print(f"Modulo: {x} % {y} = {mod}")

# Comparisons
print(f"{x} > {y}: {x > y}")
print(f"{x} < {y}: {x < y}")
print(f"{x} == {y}: {x == y}")
print(f"{x} != {y}: {x != y}")
print(f"{x} >= {y}: {x >= y}")
print(f"{x} <= {y}: {x <= y}")

# Boolean operators
a = True
b = False
print(f"True and False: {a and b}")
print(f"True or False: {a or b}")
print(f"not True: {not a}")

# String operations
name = "Scriptling"
greeting = "Hello, " + name + "!"
print(greeting)

print("✓ All basic tests passed")
