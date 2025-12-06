# Scriptling Quick Reference

## JSON Library

```python
import json

# Parse JSON string to objects
data = json.loads('{"name":"Alice","age":30}')
name = data["name"]  # "Alice"

# Convert objects to JSON string
obj = {"status": "success", "count": 42}
json_str = json.dumps(obj)
```

## Regex Library

```python
import re

# Match - returns boolean (matches at start of string only)
if re.match("[0-9]+", "123abc"):
    print("Starts with digits")

# Search - returns first match anywhere or None
email = re.search("[a-z]+@[a-z]+\.[a-z]+", "Contact: user@example.com")

# Find all - returns list
phones = re.findall("[0-9]{3}-[0-9]{4}", "555-1234 or 555-5678")

# Find all as Match objects - returns list of Match objects
matches = re.finditer("[0-9]{3}-[0-9]{4}", "555-1234 or 555-5678")
for match in matches:
    print(match.group(0))  # "555-1234", "555-5678"

# Sub - replacement (pattern, repl, string, count=0, flags=0)
text = re.sub("[0-9]+", "XXX", "Price: 100")
text = re.sub("[0-9]+", "X", "a1b2c3", 2)  # Replace only first 2

# Split - returns list (pattern, string, maxsplit=0, flags=0)
parts = re.split("[,;]", "one,two;three")

# Flags: re.I (IGNORECASE), re.M (MULTILINE), re.S (DOTALL)
if re.match("hello", "HELLO world", re.I):
    print("Case-insensitive match")
```

## Requests Library

Response: `status_code`, `text`, `headers`, `json()`, `raise_for_status()`
Supports both dict (`response["status_code"]`) and attribute access (`response.status_code`)

### Simple Requests (5 second default timeout)

```python
import requests

# GET with json() method
response = requests.get("https://api.example.com/users")
data = response.json()
print(data)

# GET with raise_for_status()
try:
    response = requests.get("https://api.example.com/data")
    response.raise_for_status()  # Raises error if 4xx/5xx
    data = response.json()
except Exception as e:
    print("Error:", e)

# Attribute access
if response.status_code == 200:
    content = response.text[:500]
    print(content)

# POST with data
import json
body = json.stringify({"name": "Alice"})
response = requests.post("https://api.example.com/users", data=body)

# Other methods
response = requests.put(url, data=body)
response = requests.delete(url)
response = requests.patch(url, data=body)

# LLM-compatible exception handling (dotted names supported)
try:
    response = requests.get(url)
    response.raise_for_status()
    content = response.text[:500]
except requests.exceptions.RequestException as e:
    print(f"Error: {e}")
```

### With Options (timeout, headers, auth)

```python
# Using keyword arguments (Python-style)
response = requests.get(url, timeout=10)

response = requests.get(url, headers={
    "Authorization": "Bearer token123",
    "Accept": "application/json"
})

response = requests.post(url, data=body, timeout=10, headers={"Content-Type": "application/json"})

# Basic Authentication
response = requests.get(url, auth=("user", "pass"))

# Legacy options dictionary (still supported)
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123"}
}
response = requests.get(url, options)
```

### Complete Example

```python
import json
import requests

# Configure options
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123"}
}

# GET request
response = requests.get("https://api.example.com/users/1", options)

if response["status"] == 200:
    user = json.parse(response["body"])
    print("User: " + user["name"])

    # Update user
    user["email"] = "new@example.com"
    body = json.stringify(user)

    update = requests.put("https://api.example.com/users/1", body, options)
    if update["status"] == 200:
        print("Updated!")
else:
    print("Error: " + str(response["status"]))
```

## Itertools Library

```python
import itertools

# Combining iterables
itertools.chain([1, 2], [3, 4])           # [1, 2, 3, 4]
itertools.zip_longest([1, 2], [3], fillvalue=0)  # [[1, 3], [2, 0]]

# Infinite iterators (use with count limit)
itertools.count(10)                        # [10, 11, 12, ...]
itertools.cycle([1, 2])                    # [1, 2, 1, 2, ...]
itertools.repeat("x", 3)                   # ["x", "x", "x"]

# Filtering
itertools.takewhile(lambda x: x < 3, [1, 2, 3, 2, 1])  # [1, 2]
itertools.dropwhile(lambda x: x < 3, [1, 2, 3, 2, 1])  # [3, 2, 1]
itertools.filterfalse(lambda x: x % 2, [1, 2, 3, 4])   # [2, 4]
itertools.compress([1, 2, 3, 4], [1, 0, 1, 0])         # [1, 3]

# Slicing and batching
itertools.islice([0, 1, 2, 3, 4], 1, 4)    # [1, 2, 3]
itertools.batched([1, 2, 3, 4, 5], 2)      # [[1, 2], [3, 4], [5]]
itertools.pairwise([1, 2, 3, 4])           # [[1, 2], [2, 3], [3, 4]]

# Combinatorics
itertools.permutations([1, 2, 3])          # All orderings
itertools.combinations([1, 2, 3], 2)       # All pairs
itertools.product([1, 2], ["a", "b"])      # Cartesian product

# Accumulate
itertools.accumulate([1, 2, 3, 4])         # [1, 3, 6, 10]
```

## Collections Library

```python
import collections

# Counter - count element occurrences
counter = collections.Counter([1, 1, 2, 3, 3, 3])  # {1: 2, 2: 1, 3: 3}
collections.most_common(counter, 2)                 # [(3, 3), (1, 2)]

# deque - double-ended queue
d = collections.deque([1, 2, 3])
collections.deque_appendleft(d, 0)          # [0, 1, 2, 3]
collections.deque_popleft(d)                # Returns 0, d is [1, 2, 3]
collections.deque_rotate(d, 1)              # Rotate right

# namedtuple - factory for dict with named fields
Point = collections.namedtuple("Point", ["x", "y"])
p = Point(1, 2)
print(p.x, p.y)                                 # 1 2
p = Point(10, 20)
p["x"]                                      # 10

# ChainMap - merge multiple dicts
defaults = {"a": 1, "b": 2}
overrides = {"b": 20, "c": 3}
cm = collections.ChainMap(overrides, defaults)
cm["a"]                                     # 1 (from defaults)
cm["b"]                                     # 20 (from overrides)

# defaultdict - dict with default factory
d = collections.defaultdict(list)
d["key"].append(1)                              # Creates [] and appends 1
```

## Copy Library

```python
import copy

# Shallow copy - new container, shared nested objects
original = [[1, 2], [3, 4]]
shallow = copy.copy(original)

# Deep copy - completely independent copy
deep = copy.deepcopy(original)

# For flat structures, both work the same
simple = [1, 2, 3]
copied = copy.copy(simple)
```

## Core Built-in Functions

Always available without importing:

```python
# I/O
print("Hello")
input("Enter name: ")         # Read user input (returns string)

# Type conversions
str(42)           # "42"
int("42")         # 42
float("3.14")     # 3.14
bool(0)           # False (bool conversion)
type(42)          # "INTEGER"
type("hello")     # "STRING"
list("abc")       # ["a", "b", "c"]
dict()            # {}
tuple([1, 2, 3])  # (1, 2, 3)
tuple([1, 2, 3])  # (1, 2, 3)
set([1, 2, 2, 3]) # {1, 2, 3} (unique elements)

# Type checking
callable(len)                 # True (is function)
callable(42)                  # False
isinstance(42, "int")         # True
isinstance("hi", "str")       # True
isinstance(None, "NoneType")  # True

# Math operations
abs(-5)           # 5
min(3, 1, 2)      # 1
max(3, 1, 2)      # 3
round(3.7)        # 4
round(3.14159, 2) # 3.14
pow(2, 10)        # 1024
pow(2, 10, 1000)  # 24 (modular: 2^10 % 1000)
divmod(17, 5)     # (3, 2) - quotient and remainder

# Number formatting
hex(255)          # "0xff"
bin(10)           # "0b1010"
oct(8)            # "0o10"

# Character conversion
chr(65)           # "A"
ord("A")          # 65

# Iteration utilities (return iterators)
list(enumerate(["a", "b"]))    # [(0, "a"), (1, "b")]
list(zip([1, 2], ["a", "b"]))  # [(1, "a"), (2, "b")]
list(reversed([1, 2, 3]))      # [3, 2, 1]
list(map(lambda x: x*2, [1,2,3]))   # [2, 4, 6]
list(filter(lambda x: x>1, [1,2,3]))# [2, 3]
any([False, True, False])     # True
all([True, True, True])       # True

# String operations
len("hello")                        # 5
upper("hello")                      # "HELLO"
lower("HELLO")                      # "hello"
split("a,b,c", ",")                # ["a", "b", "c"]
join(["a", "b"], "-")              # "a-b"
replace("hello", "l", "L")         # "heLLo"

# Triple-quoted and raw strings
multi = '''Line1
Line2
'''
raw = r"C:\\path\\to\\file"

# List operations
numbers = [1, 2, 3]
len(numbers)                        # 3
numbers.append(4)                   # numbers is now [1, 2, 3, 4]
sum([1, 2, 3, 4])                   # 10
sorted([3, 1, 2])                   # [1, 2, 3]
sorted([3, 1, 2], reverse=True)     # [3, 2, 1]
sorted(["banana", "apple"], len)    # Sort with key function

# Range (returns iterator)
list(range(5))                      # [0, 1, 2, 3, 4]
list(range(2, 5))                   # [2, 3, 4]
list(range(0, 10, 2))               # [0, 2, 4, 6, 8]

# Slice operations
lst = [0, 1, 2, 3, 4, 5]
lst[1:4]                    # [1, 2, 3]
lst[::2]                    # [0, 2, 4]
lst[::-1]                   # [5, 4, 3, 2, 1, 0]

# Using slice() builtin
s = slice(1, 4)
lst[s]                       # [1, 2, 3]
s = slice(None, None, -1)
lst[s]                       # [5, 4, 3, 2, 1, 0]
s = slice(-3, None)
lst[s]                       # [3, 4, 5]

# Dictionary methods (return views)
person = {"name": "Alice", "age": 30}
list(person.keys())                 # ["name", "age"]
list(person.values())               # ["Alice", 30]
list(person.items())                # [("name", "Alice"), ("age", 30)]
```

## Control Flow

```python
# If/elif/else
if x > 10:
    print("large")
elif x > 5:
    print("medium")
else:
    print("small")

# While loop
while x > 0:
    x = x - 1

# For loop
for item in [1, 2, 3]:
    print(item)

# For with range
for i in range(5):
    print(i)

# Break and continue
for i in range(10):
    if i == 3:
        continue  # Skip 3
    if i == 7:
        break     # Stop at 7
    print(i)
```

## Assert Statement

```python
# Assert condition (raises AssertionError if false)
assert x > 0
assert len(data) > 0

# Assert with message
assert x > 0, "x must be positive"
assert user is not None, "User not found"
```

## String Methods

```python
# Case conversion
"hello".upper()              # "HELLO"
"HELLO".lower()              # "hello"
"hello".capitalize()         # "Hello"
"hello world".title()        # "Hello World"
"Hello World".swapcase()     # "hELLO wORLD"

# Finding and checking
"hello".startswith("he")     # True
"hello".endswith("lo")       # True
"hello".find("l")            # 2 (first index or -1)
"hello".count("l")           # 2
"hello".isalpha()            # True
"12345".isdigit()            # True
"abc123".isalnum()           # True

# Splitting and joining
"a,b,c".split(",")           # ["a", "b", "c"]
", ".join(["a", "b"])        # "a, b"
"hello\nworld".splitlines()  # ["hello", "world"]
"hello-world".partition("-") # ("hello", "-", "world")
"a-b-c".rpartition("-")      # ("a-b", "-", "c")

# Trimming and padding
"  hello  ".strip()          # "hello"
"  hello  ".lstrip()         # "hello  "
"  hello  ".rstrip()         # "  hello"
"hello".center(11)           # "   hello   "
"hello".ljust(10)            # "hello     "
"hello".rjust(10)            # "     hello"
"5".zfill(3)                 # "005"

# Replacing and removing
"hello".replace("l", "L")    # "heLLo"
"TestCase".removeprefix("Test")    # "Case"
"file.py".removesuffix(".py")      # "file"

# Encoding
"ABC".encode()               # [65, 66, 67] (byte values)
```

## Functions

```python
def add(a, b):
    return a + b

result = add(5, 3)  # 8

# Recursion
def fibonacci(n):
    if n <= 1:
        return n
    else:
        return fibonacci(n - 1) + fibonacci(n - 2)

# Variadic arguments (*args)
def sum_all(*args):
    total = 0
    for n in args:
        total += n
    return total

sum_all(1, 2, 3)  # 6
```

## Error Handling

```python
# Try/except
try:
    result = 10 / 0
except:
    result = 0

# Try/finally
try:
    data = process()
finally:
    cleanup()

# Try/except/finally
try:
    response = requests.get(url, options)
    data = json.parse(response["body"])
except:
    data = None
finally:
    print("Done")

# Raise errors
def validate(x):
    if x < 0:
        raise "Value must be positive"
    return x

try:
    validate(-5)
except:
    print("Validation failed")

# Multiple assignment (tuple unpacking)
a, b = [1, 2]
x, y = [y, x]  # Swap variables
```

## Data Types

```python
# Integer
x = 42

# Float
pi = 3.14

# String
name = "Alice"

# Boolean
flag = True
done = False

# None
result = None

# List
numbers = [1, 2, 3, 4, 5]
mixed = [1, "two", 3.0, True]

# Dictionary
person = {
    "name": "Alice",
    "age": 30,
    "active": True
}

# Set
unique = set([1, 2, 2, 3])  # {1, 2, 3}

# Indexing
first = numbers[0]      # 1
last = numbers[4]       # 5
value = person["name"]  # "Alice"

# Slicing
numbers[1:3]            # [2, 3]
numbers[:3]             # [1, 2, 3]
numbers[3:]             # [4, 5]
```

## Operators

```python
# Arithmetic
x + y    # Addition
x - y    # Subtraction
x * y    # Multiplication
x ** y   # Exponentiation (power, e.g., 2**3 = 8)
x / y    # Division (always returns float, e.g., 5 / 2 = 2.5)
x // y   # Floor division (integer division, e.g., 7 // 2 = 3)
x % y    # Modulo (remainder, e.g., 5 % 2 = 1)

# Augmented assignment
x += 5   # x = x + 5
x -= 3   # x = x - 3
x *= 2   # x = x * 2
x /= 4   # x = x / 4
x //= 3  # x = x // 3 (floor division)
x &= 3   # x = x & 3 (bitwise AND)
x |= 3   # x = x | 3 (bitwise OR)
x ^= 3   # x = x ^ 3 (bitwise XOR)
x <<= 2  # x = x << 2 (left shift)
x >>= 2  # x = x >> 2 (right shift)

# Bitwise (integers only)
~x       # Bitwise NOT (e.g., ~5 = -6)
x & y    # Bitwise AND (e.g., 12 & 10 = 8)
x | y    # Bitwise OR (e.g., 12 | 10 = 14)
x ^ y    # Bitwise XOR (e.g., 12 ^ 10 = 6)
x << y   # Left shift (e.g., 5 << 2 = 20)
x >> y   # Right shift (e.g., 20 >> 2 = 5)

# Comparison
x == y   # Equal
x != y   # Not equal
x < y    # Less than
x > y    # Greater than
x <= y   # Less than or equal
x >= y   # Greater than or equal

# Logical
x and y  # Logical AND
x or y   # Logical OR
not x    # Logical NOT

# String concatenation
"Hello" + " " + "World"  # "Hello World"
```

## Best Practices

1. **Always check HTTP status codes**
   ```python
   if response["status"] == 200:
       # Process response
   else:
       print("Error: " + str(response["status"]))
   ```

2. **Use options dictionary for clarity**
   ```python
   options = {"timeout": 10}
   response = requests.get(url, options)
   ```

3. **Parse JSON responses**
   ```python
   data = json.parse(response["body"])
   ```

4. **Use descriptive variable names**
   ```python
   user_count = 10  # Good
   x = 10           # Bad
   ```

5. **Set appropriate timeouts**
   ```python
   # Default is 5 seconds, increase for slow APIs
   options = {"timeout": 30}
   ```
