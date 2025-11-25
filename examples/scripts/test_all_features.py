# Test: Comprehensive test of all features

print("=== Comprehensive Feature Test ===")

# 1. Variables and types
x = 42
s = "test"
f = 3.14
b = True
print(f"Variables: x={x}, s={s}, f={f}, b={b}")

# 2. Arithmetic
result = (10 + 5) * 2 - 3
print(f"Arithmetic: (10 + 5) * 2 - 3 = {result}")

# 3. Lists and dicts
items = [1, 2, 3]
data = {"key": "value"}
print(f"Collections: items={items}, data={data}")

# 4. Control flow
if x > 40:
    print("Control flow: x > 40")

# 5. Loops
total = 0
for i in range(5):
    total = total + i
print(f"Loop sum: {total}")

# 6. Functions
def multiply(a, b):
    return a * b

result = multiply(6, 7)
print(f"Function result: multiply(6, 7) = {result}")

# 7. Error handling
try:
    value = 10 / 2
    print(f"Error handling: 10 / 2 = {value}")
except Exception as e:
    print(f"Error: {e}")

# 8. String methods
text = "hello"
upper = upper(text)
print(f"String method: upper('{text}') = {upper}")

# 9. List methods
nums = [1, 2, 3]
nums.append(4)
print(f"List method: after append = {nums}")

# 10. Imports
import math
sqrt_val = math.sqrt(16)
print(f"Import: math.sqrt(16) = {sqrt_val}")

print("✓ All comprehensive tests passed")
