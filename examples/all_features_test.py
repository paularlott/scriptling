import("json")
import("http")

print("=== Testing All Scriptling Features ===\n")

# 1. Variables and augmented assignment
x = 10
x += 5
print("Augmented assignment:", x)

# 2. Booleans
active = True
print("Boolean:", active)

# 3. If/elif/else
score = 85
if score >= 90:
    grade = "A"
elif score >= 80:
    grade = "B"
else:
    grade = "C"
print("Grade:", grade)

# 4. Break and continue
print("\nLoop control:")
for i in [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]:
    if i > 7:
        break
    if i % 2 == 0:
        continue
    print("  Odd number:", i)

# 5. Pass statement
for i in [1, 2, 3]:
    if i == 2:
        pass
    else:
        print("  Not 2:", i)

# 6. Functions
def fibonacci(n):
    if n <= 1:
        return n
    else:
        return fibonacci(n - 1) + fibonacci(n - 2)

print("\nFibonacci(7):", fibonacci(7))

# 7. Lists and dicts
numbers = [1, 2, 3]
append(numbers, 4)
person = {"name": "Alice", "age": "30"}
print("List:", numbers)
print("Dict:", person["name"])

# 8. JSON
json_str = json.stringify({"test": "value"})
data = json.parse(json_str)
print("JSON roundtrip:", data["test"])

# 9. HTTP with headers
headers = {"User-Agent": "Scriptling/1.0"}
response = http.get("https://httpbin.org/status/200", headers, 10)
print("HTTP status:", response["status"])

print("\n=== All Features Working! ===")