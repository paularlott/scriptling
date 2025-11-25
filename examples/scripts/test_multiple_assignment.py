# Test: Multiple assignment

print("=== Testing Multiple Assignment ===")

# Two variables
x, y = [10, 20]
print(f"x = {x}, y = {y}")

# Three variables
a, b, c = [1, 2, 3]
print(f"a = {a}, b = {b}, c = {c}")

# Swap variables
x, y = [y, x]
print(f"After swap: x = {x}, y = {y}")

# From function return
def get_coords():
    return [100, 200]

x, y = get_coords()
print(f"From function: x = {x}, y = {y}")

# Mixed types
name, age = ["Alice", 30]
print(f"name = {name}, age = {age}")

print("✓ All multiple assignment tests passed")
