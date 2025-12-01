# Scriptling Performance Benchmark
import time

print("=== Scriptling Performance Benchmark ===")
print("")

# Arithmetic Operations
print("1. Arithmetic Operations (1000 iterations)")
start = time.perf_counter()
for i in range(1000):
    x = i + 1
    y = x * 2
    z = y - 1
end = time.perf_counter()
print("   Time: " + str(end - start) + " seconds")
print("")

# Variable Operations
print("2. Variable Operations (1000 iterations)")
start = time.perf_counter()
for i in range(1000):
    a = i
    b = a
    c = b
end = time.perf_counter()
print("   Time: " + str(end - start) + " seconds")
print("")

# List Operations
print("3. List Operations (100 iterations)")
start = time.perf_counter()
items = []
for i in range(100):
    items.append(i)
total = 0
for item in items:
    total = total + item
end = time.perf_counter()
print("   Time: " + str(end - start) + " seconds")
print("")

# Dictionary Operations
print("4. Dictionary Operations (100 iterations)")
start = time.perf_counter()
for i in range(100):
    data = {"a": 1, "b": 2, "c": 3}
    x = data["a"] + data["b"] + data["c"]
end = time.perf_counter()
print("   Time: " + str(end - start) + " seconds")
print("")

# String Operations
print("5. String Operations (100 iterations)")
start = time.perf_counter()
parts = []
for i in range(100):
    parts.append(str(i))
result = ",".join(parts)
end = time.perf_counter()
print("   Time: " + str(end - start) + " seconds")
print("")

# Function Calls
print("6. Function Calls (100 iterations)")
def add(a, b):
    return a + b

start = time.perf_counter()
total = 0
for i in range(100):
    total = total + add(i, 1)
end = time.perf_counter()
print("   Time: " + str(end - start) + " seconds")
print("")

# Fibonacci (recursive)
print("7. Fibonacci(15) - Recursive")
def fib(n):
    if n <= 1:
        return n
    return fib(n-1) + fib(n-2)

start = time.perf_counter()
result = fib(15)
end = time.perf_counter()
print("   Result: " + str(result))
print("   Time: " + str(end - start) + " seconds")
print("")

# JSON Operations
print("8. JSON Operations (100 iterations)")
import json
start = time.perf_counter()
for i in range(100):
    data = json.loads('{"name":"Alice","age":30,"active":true}')
    text = json.dumps(data)
end = time.perf_counter()
print("   Time: " + str(end - start) + " seconds")
print("")

# Time formatting
print("9. Time Formatting")
now = time.time()
local_tuple = time.localtime(now)
formatted = time.strftime("%Y-%m-%d %H:%M:%S", local_tuple)
print("   Current time: " + formatted)
parsed_tuple = time.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S")
parsed_timestamp = time.mktime(parsed_tuple)
print("   Parsed timestamp: " + str(parsed_timestamp))
print("")

print("=== Benchmark Complete ===")
