import("json")
import("http")

print("=== Complete Scriptling Feature Test ===\n")

# 1. Variables and augmented assignment
x = 10
x += 5
x *= 2
print("1. Augmented assignment:", x)

# 2. Booleans
active = True
inactive = False
print("2. Booleans:", active, "and", inactive)

# 3. If/elif/else
score = 85
if score >= 90:
    grade = "A"
elif score >= 80:
    grade = "B"
elif score >= 70:
    grade = "C"
else:
    grade = "F"
print("3. If/elif/else - Grade:", grade)

# 4. Range function
print("4. Range function:")
for i in range(3):
    print("  ", i)

# 5. Slice notation
numbers = [0, 1, 2, 3, 4, 5]
print("5. Slice notation:", numbers[1:4])
text = "Hello World"
print("   String slice:", text[0:5])

# 6. Break and continue
print("6. Loop control:")
for i in range(10):
    if i > 5:
        break
    if i % 2 == 0:
        continue
    print("   Odd:", i)

# 7. Pass statement
for i in range(3):
    if i == 1:
        pass
    else:
        print("7. Pass test:", i)

# 8. Dictionary methods
person = {"name": "Alice", "age": "30", "city": "NYC"}
print("8. Dict keys:", keys(person))
print("   Dict values:", values(person))
print("   Iterating items:")
for item in items(person):
    print("     ", item[0], "=", item[1])

# 9. Functions with recursion
def factorial(n):
    if n <= 1:
        return 1
    else:
        return n * factorial(n - 1)

print("9. Factorial(5):", factorial(5))

# 10. Lists and operations
nums = [1, 2, 3]
append(nums, 4)
print("10. List operations:", nums)
print("    List slice:", nums[1:3])

# 11. JSON
data = {"test": "value", "number": "42"}
json_str = json.stringify(data)
parsed = json.parse(json_str)
print("11. JSON roundtrip:", parsed["test"])

# 12. HTTP with headers
headers = {"User-Agent": "Scriptling/1.0", "Accept": "application/json"}
response = http.get("https://httpbin.org/status/200", headers, 10)
print("12. HTTP with headers - Status:", response["status"])

# 13. Complex loop with range and slice
result = 0
for i in range(1, 6):
    result += i
print("13. Sum of range(1,6):", result)

# 14. Nested structures
matrix = [[1, 2, 3], [4, 5, 6], [7, 8, 9]]
print("14. Matrix access:", matrix[1][2])
print("    Matrix row slice:", matrix[0][1:3])

print("\n=== All Features Working Perfectly! ===")