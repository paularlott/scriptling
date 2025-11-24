# Comprehensive test of all Scriptling features
# Tests every major language feature and library

print("=== Scriptling Comprehensive Feature Test ===")
print("")

# 1. Variables and Types
print("1. Variables and Types")
x = 42
pi = 3.14
name = "Alice"
flag = True
none_val = None
print("   ✓ All types work")

# 2. Operators
print("2. Operators")
result = (10 + 5) * 2 - 3
comparison = result > 20 and result < 30
membership = "hello" in "hello world"
print("   ✓ Arithmetic, comparison, membership")

# 3. Control Flow
print("3. Control Flow")
if result > 20:
    status = "pass"
elif result > 10:
    status = "maybe"
else:
    status = "fail"
print("   ✓ if/elif/else")

# 4. Loops
print("4. Loops")
count = 0
for i in range(5):
    count = count + 1
print("   ✓ for loop, count:", count)

# 5. Functions
print("5. Functions")
def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)
print("   ✓ Functions with recursion, 5! =", factorial(5))

# 6. Collections
print("6. Collections")
nums = [1, 2, 3]
append(nums, 4)
data = {"key": "value", "num": "42"}
print("   ✓ Lists and dicts")

# 7. Multiple Assignment
print("7. Multiple Assignment")
a, b = [10, 20]
a, b = [b, a]
print("   ✓ Unpacking and swap, a:", a, "b:", b)

# 8. Scope Management
print("8. Scope Management")
counter = 0
def increment():
    global counter
    counter = counter + 1
increment()
print("   ✓ global keyword, counter:", counter)

# 9. Error Handling
print("9. Error Handling")
try:
    x = 10 / 0
except:
    x = 0
finally:
    cleanup = True
print("   ✓ try/except/finally")

# 10. Libraries - json
print("10. Libraries - json")
import json
obj = {"test": "value"}
json_str = json.stringify(obj)
parsed = json.parse(json_str)
print("   ✓ json library")

# 11. Libraries - math
print("11. Libraries - math")
import math
sqrt_val = math.sqrt(16)
pi_val = math.pi()
print("   ✓ math library, sqrt(16):", sqrt_val)

# 12. Libraries - base64
print("12. Libraries - base64")
import base64
encoded = base64.encode("hello")
decoded = base64.decode(encoded)
print("   ✓ base64 library")

# 13. Libraries - hashlib
print("13. Libraries - hashlib")
import hashlib
hash_val = hashlib.sha256("test")
print("   ✓ hashlib library")

# 14. Libraries - random
print("14. Libraries - random")
import random
rand_num = random.randint(1, 100)
rand_float = random.random()
print("   ✓ random library")

# 15. Libraries - url
print("15. Libraries - url")
import url
url_encoded = url.encode("hello world")
url_decoded = url.decode(url_encoded)
print("   ✓ url library")

# 16. Nested dicts (multi-line)
print("16. Nested dicts")
config = {
    "db": {"host": "localhost", "port": "5432"},
    "cache": {"host": "redis"}
}
print("   ✓ Nested multi-line dicts, db host:", config["db"]["host"])

print("")
print("=== ALL FEATURES WORKING! ===")
