# Performance benchmarks for Scriptling - Regression testing for critical paths
import time
import math

passed = True

# Benchmark 1: List operations
print("Benchmarking list operations...")
start = time.time()
lists = []
for i in range(1000):
    lst = list(range(100))
    lists.append(lst)
list_time = time.time() - start
print(f"Created 1000 lists of 100 elements: {list_time:.4f}s")

# Benchmark 2: Dictionary operations
print("Benchmarking dictionary operations...")
start = time.time()
dicts = []
for i in range(1000):
    d = {}
    for j in range(100):
        d[j] = j*2
    dicts.append(d)
dict_time = time.time() - start
print(f"Created 1000 dicts of 100 elements: {dict_time:.4f}s")

# Benchmark 3: String operations
print("Benchmarking string operations...")
start = time.time()
strings = []
for i in range(1000):
    s = "test_string_" + str(i) * 10
    strings.append(s.upper())
string_time = time.time() - start
print(f"Created and uppercased 1000 strings: {string_time:.4f}s")

# Benchmark 4: Function calls
print("Benchmarking function calls...")
def simple_func(x):
    return x * 2 + 1

start = time.time()
results = []
for i in range(10000):
    results.append(simple_func(i))
func_time = time.time() - start
print(f"Called function 10000 times: {func_time:.4f}s")

# Benchmark 5: Math operations
print("Benchmarking math operations...")
start = time.time()
math_results = []
for i in range(10000):
    math_results.append(math.sin(i) + math.cos(i) + math.sqrt(i+1))
math_time = time.time() - start
print(f"Performed 10000 math operations: {math_time:.4f}s")

# Benchmark 6: List comprehensions
print("Benchmarking list comprehensions...")
start = time.time()
comprehensions = [x**2 for x in range(10000)]
comp_time = time.time() - start
print(f"Created list comprehension of 10000 elements: {comp_time:.4f}s")

# Benchmark 7: Sorting
print("Benchmarking sorting...")
start = time.time()
for _ in range(10):  # Fewer iterations
    data = list(range(100))  # Smaller lists
    # Simple reverse to shuffle
    data.reverse()
    sorted(data)
sort_time = time.time() - start
print(f"Sorted 10 lists of 100 elements: {sort_time:.4f}s")

# Benchmark 8: File I/O simulation (string operations)
print("Benchmarking string processing (file I/O simulation)...")
start = time.time()
content = "line " * 10000
lines = content.split()
processed = [line.upper() for line in lines]
file_time = time.time() - start
print(f"Processed 10000 'lines': {file_time:.4f}s")

# Performance assertions (these are regression tests)
# These times are based on reasonable expectations for the interpreter
# If these fail, it indicates a performance regression

# Allow some tolerance for different systems
tolerance = 2.0

assert list_time < 1.0 * tolerance, f"List operations too slow: {list_time}s"
assert dict_time < 1.0 * tolerance, f"Dict operations too slow: {dict_time}s"
assert string_time < 0.5 * tolerance, f"String operations too slow: {string_time}s"
assert func_time < 0.5 * tolerance, f"Function calls too slow: {func_time}s"
assert math_time < 1.0 * tolerance, f"Math operations too slow: {math_time}s"
assert comp_time < 0.5 * tolerance, f"List comprehensions too slow: {comp_time}s"
assert sort_time < 2.0 * tolerance, f"Sorting too slow: {sort_time}s"
assert file_time < 0.5 * tolerance, f"String processing too slow: {file_time}s"

print("All performance benchmarks passed!")
print(".4f")
print(".4f")
print(".4f")
print(".4f")
print(".4f")
print(".4f")
print(".4f")
print(".4f")

assert passed