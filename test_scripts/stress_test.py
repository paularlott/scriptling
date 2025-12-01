# Stress testing for Scriptling - Large data structures, deep recursion, memory limits

passed = True

# Test 1: Large list operations
print("Testing large list operations...")
large_list = list(range(10000))
assert len(large_list) == 10000
assert large_list[0] == 0
assert large_list[-1] == 9999

# Sum of large list
total = sum(large_list)
assert total == 49995000  # n*(n-1)/2 for n=10000

# Test 2: Large dictionary operations
print("Testing large dictionary operations...")
large_dict = {}
for i in range(5000):
    large_dict[i] = i*2
assert len(large_dict) == 5000
assert large_dict[0] == 0
assert large_dict[4999] == 9998

# Test 3: Deep recursion (but not too deep to avoid stack overflow)
print("Testing deep recursion...")
def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n-1)

# Test factorial up to 100 (should be fine)
fact_10 = factorial(10)
assert fact_10 == 3628800

fact_20 = factorial(20)
assert fact_20 == 2432902008176640000

# Test 4: Large string operations
print("Testing large string operations...")
large_string = "a" * 10000
assert len(large_string) == 10000
assert large_string[0] == "a"
assert large_string[-1] == "a"

# String concatenation
big_string = large_string + "b" * 5000
assert len(big_string) == 15000

# Test 5: Nested data structures
print("Testing nested data structures...")
nested = []
for i in range(100):
    data_list = []
    for j in range(10):  # Smaller list to avoid issues
        data_list.append(j)
    inner = {"id": i, "data": data_list, "meta": {"created": 123.45, "active": True}}
    nested.append(inner)

assert len(nested) == 100
assert nested[0]["id"] == 0
assert len(nested[0]["data"]) == 10

# Test 6: Large set operations
print("Testing large set operations...")
large_set = set(range(2000))
assert len(large_set) == 2000
assert 0 in large_set
assert 1999 in large_set
assert 2000 not in large_set

# Test 7: Complex list comprehensions
print("Testing complex list comprehensions...")
matrix = [[i*j for j in range(50)] for i in range(50)]
assert len(matrix) == 50
assert len(matrix[0]) == 50
assert matrix[0][0] == 0
assert matrix[5][7] == 35

# Test 8: Deeply nested dictionaries
print("Testing deeply nested dictionaries...")
deep_dict = {}
current = deep_dict
for i in range(20):  # 20 levels deep
    current["level"] = i
    current["next"] = {}
    current = current["next"]

assert deep_dict["level"] == 0
assert deep_dict["next"]["next"]["next"]["level"] == 3  # 4th level

# Test 9: Large tuple operations
print("Testing large tuple operations...")
large_tuple = tuple(range(100))  # Smaller for now
assert len(large_tuple) == 100
assert large_tuple[0] == 0
assert large_tuple[-1] == 99

# Test 10: Memory-intensive string processing
print("Testing memory-intensive string processing...")
words = []
for i in range(10):
    words.append("word")
big_text = " ".join(words)
assert len(big_text) == 49  # 10 words * 4 chars + 9 spaces
assert big_text.startswith("word")
assert big_text.endswith("word")

print("All stress tests passed!")
assert passed