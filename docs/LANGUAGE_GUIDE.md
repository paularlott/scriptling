# Scriptling Language Guide

## For LLMs and Developers

This document provides a complete reference for the Scriptling programming language - a minimal Python-like scripting language designed for embedding in Go applications and REST API automation.

## Language Overview

Scriptling is a dynamically-typed, interpreted language with Python-inspired syntax. It supports:
- Variables, functions, and control flow
- Lists and dictionaries
- String manipulation
- JSON processing
- HTTP/REST API calls
- Go interoperability

## Differences from Python

While Scriptling is inspired by Python, it has some key differences:

- **No Nested Classes**: Classes cannot be defined within other classes.
- **Simplified Scope**: `nonlocal` and `global` keywords work slightly differently.
- **Go Integration**: Designed primarily for embedding in Go, with direct type mapping.
- **Sandboxed**: No direct access to filesystem or network unless explicitly enabled via libraries.

## Syntax Rules

### Indentation
Scriptling uses **Python-style indentation** (4 spaces recommended) to define code blocks:
```python
if x > 5:
    print("yes")    # 4 spaces indent
    y = 10
```

### Comments
```python
# Single-line comments only
x = 5  # Inline comments supported
```

### Triple-Quoted and Raw Strings
Scriptling supports single- and double-quoted strings, triple-quoted strings for multi-line text, and raw-string prefixes `r`/`R` which are useful for regular expressions.

### Case Sensitivity
- Keywords: lowercase (`if`, `while`, `def`, `return`)
- Booleans: `True`, `False` (capitalized)
- Variables: case-sensitive (`myVar` ≠ `myvar`)

### Multiline Syntax
Scriptling supports multiline definitions for lists, dictionaries, function calls, and function definitions. Indentation is ignored inside parentheses, brackets, and braces.

```python
# Multiline list
numbers = [
    1,
    2,
    3,
]

# Multiline dictionary
person = {
    "name": "Alice",
    "age": 30,
}

# Multiline function call
result = my_function(
    arg1,
    arg2,
    key="value"
)
```

### Trailing Commas
Trailing commas are allowed in lists, dictionaries, function calls, and function definitions. This makes it easier to add or remove items in multiline structures.

## Data Types

### Integer
```python
x = 42
y = -10
z = 0
```

### Float
```python
pi = 3.14
temp = -273.15
```

### String
```python
name = "Alice"
message = 'Hello'  # Single or double quotes
```

### Boolean
```python
flag = True
done = False
```

### List
```python
numbers = [1, 2, 3, 4, 5]
mixed = [1, "two", 3.0, True]
nested = [1, [2, 3], 4]
empty = []
```

### Dictionary
```python
person = {"name": "Alice", "age": "30"}
config = {"host": "localhost", "port": "8080"}
empty = {}
```

### Set
```python
numbers = set([1, 2, 3])
unique = set([1, 2, 2, 3])  # {1, 2, 3}
empty = set()
```

### None/Null
Represented as `None` (not directly creatable, returned by functions with no return value)

## Operators

### Arithmetic
```python
x + y    # Addition
x - y    # Subtraction
x * y    # Multiplication (numbers) or repetition (string * int)
x ** y   # Exponentiation (power)
x / y    # Division (always returns float)
x // y   # Floor division (integer division, e.g., 7 // 2 = 3)
x % y    # Modulo
```

### Augmented Assignment
```python
x += y   # x = x + y
x -= y   # x = x - y
x *= y   # x = x * y
x /= y   # x = x / y
x //= y  # x = x // y (floor division)
x %= y   # x = x % y
x &= y   # x = x & y (bitwise AND)
x |= y   # x = x | y (bitwise OR)
x ^= y   # x = x ^ y (bitwise XOR)
x <<= y  # x = x << y (left shift)
x >>= y  # x = x >> y (right shift)
```

### Bitwise Operators
Bitwise operators work on integers at the binary level, following Python's behavior:

```python
~x       # Bitwise NOT (one's complement)
x & y    # Bitwise AND
x | y    # Bitwise OR
x ^ y    # Bitwise XOR (exclusive or)
x << y   # Left shift (multiply by 2^y)
x >> y   # Right shift (divide by 2^y, floor division)

# Examples
print(~5)        # -6 (bitwise NOT using two's complement)
print(12 & 10)   # 8  (1100 & 1010 = 1000)
print(12 | 10)   # 14 (1100 | 1010 = 1110)
print(12 ^ 10)   # 6  (1100 ^ 1010 = 0110)
print(5 << 2)    # 20 (5 * 2^2 = 5 * 4)
print(20 >> 2)   # 5  (20 / 2^2 = 20 / 4)

# Augmented assignment
x = 12
x &= 10  # x is now 8
x |= 6   # x is now 14
x ^= 3   # x is now 13
x <<= 1  # x is now 26
x >>= 2  # x is now 6

# Practical use cases
# Extract lower 4 bits
value = 170  # 0b10101010
lower_bits = value & 15  # 10 (0b1010)

# Set specific bits
flags = 0
flags |= 4   # Set bit 2
flags |= 8   # Set bit 3

# Toggle bits
state = 15   # 0b1111
state ^= 5   # Toggle bits 0 and 2, result: 10 (0b1010)

# Fast multiplication/division by powers of 2
fast_mult = 7 << 3   # 7 * 8 = 56
fast_div = 56 >> 3   # 56 / 8 = 7
```

**Note**: Bitwise operators only work with integers. Negative numbers use two's complement representation, matching Python's behavior.

### Comparison
```python
x == y   # Equal
x != y   # Not equal
x < y    # Less than
x > y    # Greater than
x <= y   # Less than or equal
x >= y   # Greater than or equal
```

### Boolean/Logical
```python
x and y  # Logical AND - returns first falsy value or last value
x or y   # Logical OR - returns first truthy value or last value
not x    # Logical NOT

# Short-circuit assignment (Python-style)
config = user_config or default_config  # Use default if user_config is falsy
value = x and y  # Returns x if x is falsy, otherwise y

# Falsy values: 0, 0.0, "", [], {}, None, False
# All other values are truthy
```

### Chained Comparisons
```python
# Chained comparisons work like mathematical notation
1 < x < 10        # Equivalent to: 1 < x and x < 10
x <= y <= z       # Equivalent to: x <= y and y <= z
a == b == c       # Equivalent to: a == b and b == c

# Practical examples
if 18 <= age <= 65:
    print("Working age")

if 0 < score < 100:
    print("Valid score")
```

### Precedence (highest to lowest)
1. Parentheses `()`
2. Function calls, indexing `func()`, `list[0]`
3. Exponentiation `**`
4. Unary `-`, `not`, `~`
5. `*`, `/`, `%`
6. `+`, `-`
7. `<<`, `>>` (bitwise shift)
8. `&` (bitwise AND)
9. `^` (bitwise XOR)
10. `|` (bitwise OR)
11. `<`, `>`, `<=`, `>=`
12. `==`, `!=`
13. `and` (logical AND)
14. `or` (logical OR)

## Variables

### Assignment
```python
x = 10
name = "Alice"
result = x * 2
```

### No Declaration Required
Variables are created on first assignment.

### Scope
- Global scope: Variables defined at module level
- Function scope: Variables defined in functions (including parameters)
- No block scope (if/while blocks share outer scope)

## Control Flow

### If/Elif/Else Statement
```python
if condition:
    # code block
elif other_condition:
    # code block
elif another_condition:
    # code block
else:
    # code block
```

### Examples
```python
# Simple if/else
if x > 10:
    print("large")
else:
    print("small")

# Multiple conditions with elif
score = 85
if score >= 90:
    print("Grade: A")
elif score >= 80:
    print("Grade: B")
elif score >= 70:
    print("Grade: C")
else:
    print("Grade: F")
```

### While Loop
```python
counter = 0
while counter < 10:
    print(counter)
    counter = counter + 1
```

### For Loop
```python
# Iterate over list
for item in [1, 2, 3]:
    print(item)

# Iterate over string
for char in "hello":
    print(char)

# Iterate over variable
numbers = [10, 20, 30]
for num in numbers:
    print(num)
```

### Loop Control
```python
# break - exit loop immediately
for i in [1, 2, 3, 4, 5]:
    if i == 3:
        break
    print(i)  # Prints 1, 2

# continue - skip to next iteration
for i in [1, 2, 3, 4, 5]:
    if i == 3:
        continue
    print(i)  # Prints 1, 2, 4, 5

# pass - do nothing (placeholder)
for i in [1, 2, 3]:
    if i == 2:
        pass  # Placeholder for future code
    else:
        print(i)
```

## Functions

### Definition
```python
def function_name(param1, param2):
    # function body
    return result
```

### Keyword Arguments

Functions can be called with keyword arguments, which can be mixed with positional arguments:

```python
def greet(name, greeting="Hello", punctuation="!"):
    print(greeting + ", " + name + punctuation)

# Positional arguments
greet("World")  # Hello, World!

# Keyword arguments
greet(name="Alice")  # Hello, Alice!
greet(greeting="Hi", name="Bob")  # Hi, Bob!

# Mixed positional and keyword
greet("Charlie", greeting="Hey")  # Hey, Charlie!
greet("Diana", punctuation=".")  # Hello, Diana.

# All keyword arguments (order doesn't matter)
greet(punctuation="?", name="Eve", greeting="Howdy")  # Howdy, Eve?
```

#### Rules:

- Positional arguments must come before keyword arguments
- Each parameter can only be specified once
- Keyword arguments work with default parameter values
- Keyword arguments work with lambda functions

### Variadic Arguments (*args)

Functions can accept a variable number of positional arguments using the `*args` syntax. The extra arguments are collected into a list.

```python
def sum_all(*args):
    total = 0
    for num in args:
        total += num
    return total

print(sum_all(1, 2, 3))      # 6
print(sum_all(1, 2, 3, 4))   # 10
```

You can mix regular parameters with `*args`:

```python
def log(level, *messages):
    prefix = "[" + level + "] "
    for msg in messages:
        print(prefix + str(msg))

log("INFO", "System started", "Ready")
# Output:
# [INFO] System started
# [INFO] Ready
```

**Note**: `*args` must come after regular parameters and default parameters. It captures all remaining positional arguments.

### Examples
```python
# Simple function
def greet(name):
    print("Hello, " + name)

# Function with return
def add(a, b):
    return a + b

# Recursive function
def factorial(n):
    if n <= 1:
        return 1
    else:
        return n * factorial(n - 1)

# No parameters
def get_pi():
    return 3.14159
```

### Calling Functions
```python
greet("Alice")
result = add(5, 3)
fact = factorial(5)
```

### Return Statement
```python
return value    # Return value
return          # Return None
# No return statement also returns None
```

## Error Handling

### Try/Except
Catch and handle errors that occur during execution:
```python
try:
    result = 10 / 0
    print("This won't print")
except:
    print("Error caught")
    result = 0
```

### Try/Finally
Execute cleanup code regardless of whether an error occurs:
```python
try:
    data = process_data()
    print("Success")
finally:
    print("Cleanup always runs")
```

### Try/Except/Finally
Combine error handling with cleanup:
```python
try:
    response = requests.get(url, options)
    data = json.parse(response["body"])
except:
    print("Request failed")
    data = None
finally:
    print("Request complete")
```

### Raise Statement
Raise custom errors:
```python
def check_positive(n):
    if n < 0:
        raise "Value must be positive"
    return n * 2

try:
    result = check_positive(-5)
except:
    print("Caught error")
```

### Nested Try/Except
```python
try:
    try:
        risky_operation()
    except:
        print("Inner error")
        raise "Re-raising"
except:
    print("Outer error")
```

### Assert Statement
Test conditions and raise errors when they fail:
```python
# Basic assert - raises AssertionError if condition is False
assert x > 0

# Assert with optional error message
assert x > 0, "x must be positive"

# Common use cases
assert len(data) > 0, "Data cannot be empty"
assert user is not None, "User not found"
assert response.status_code == 200, "Request failed"

# Use in functions for validation
def divide(a, b):
    assert b != 0, "Cannot divide by zero"
    return a / b
```

### Error Handling with HTTP
```python
import json
import requests

try:
    options = {"timeout": 5}
    response = requests.get("https://api.example.com/data", options)

    if response["status"] != 200:
        raise "HTTP error: " + str(response["status"])

    data = json.parse(response["body"])
    print("Success: " + str(len(data)))
except:
    print("Request failed")
    data = []
finally:
    print("Cleanup")
```

## Lists

### Creation
```python
numbers = [1, 2, 3, 4, 5]
empty = []
```

### Indexing (0-based)
```python
first = numbers[0]    # 1
last = numbers[4]     # 5
```

### Operations
```python
len(numbers)              # Get length: 5
append(numbers, 6)        # Modifies numbers in-place
print(numbers)            # [1, 2, 3, 4, 5, 6]
```

### Iteration
```python
for num in numbers:
    print(num)
```

### Nested Lists
```python
matrix = [[1, 2], [3, 4]]
value = matrix[0][1]  # 2
```

## Dictionaries

### Creation
```python
person = {"name": "Alice", "age": "30", "city": "NYC"}
empty = {}
```

### Access
```python
name = person["name"]     # "Alice"
age = person["age"]       # "30"
```

### Operations
```python
len(person)  # Get number of keys: 3
```

### Iteration
```python
# Iterate over keys
for key in keys(person):
    print(key, person[key])

# Iterate over key-value pairs
for item in items(person):
    print(item[0], item[1])
```

### Notes
- Keys must be strings
- Values can be any type
- Missing keys return `None`

## Built-in Functions

### I/O
```python
print(value)              # Print to stdout
print("Hello", name)      # Multiple arguments
input("Prompt: ")         # Read user input (returns string)
```

### Type Conversions
```python
str(42)                   # "42"
int("42")                 # 42
int(3.14)                 # 3
float("3.14")             # 3.14
float(42)                 # 42.0
bool(0)                   # False
bool(1)                   # True
bool("")                  # False
bool("hello")             # True
type(42)                  # "INTEGER"
type(3.14)                # "FLOAT"
type("hello")             # "STRING"
type([1, 2])              # "LIST"
type({"a": "b"})          # "DICT"
type(True)                # "BOOLEAN"
list("abc")               # ["a", "b", "c"]
dict()                    # {}
tuple([1, 2, 3])          # (1, 2, 3)
set([1, 2, 2, 3])         # {1, 2, 3} (unique elements, returns set)
```

### Math Functions (built-in)
```python
abs(-5)                   # 5
min(3, 1, 2)              # 1
max(3, 1, 2)              # 3
round(3.7)                # 4
round(3.14159, 2)         # 3.14
pow(2, 10)                # 1024
pow(2, 10, 1000)          # 24 (modular: 2^10 % 1000)
divmod(17, 5)             # (3, 2) - returns (quotient, remainder)
```

### Number Formatting
```python
hex(255)                  # "0xff"
hex(-255)                 # "-0xff"
bin(10)                   # "0b1010"
bin(-10)                  # "-0b1010"
oct(8)                    # "0o10"
oct(-8)                   # "-0o10"
```

### Type Checking
```python
callable(len)             # True (is a function)
callable(42)              # False
callable(lambda x: x)     # True

isinstance(42, "int")     # True
isinstance(3.14, "float") # True
isinstance("hi", "str")   # True
isinstance([1, 2], "list")# True
isinstance({"a": 1}, "dict") # True
isinstance(True, "bool")  # True
isinstance(None, "NoneType") # True
isinstance((1, 2), "tuple") # True
```

### Character Conversion
```python
chr(65)                   # "A"
ord("A")                  # 65
```

### Iteration Utilities
```python
# These return iterators (lazy evaluation)
enumerate(["a", "b"])            # Iterator: (0, "a"), (1, "b")
zip([1, 2], ["a", "b"])          # Iterator: (1, "a"), (2, "b")
reversed([1, 2, 3])              # Iterator: 3, 2, 1
map(lambda x: x*2, [1, 2, 3])    # Iterator: 2, 4, 6
filter(lambda x: x > 1, [1, 2, 3]) # Iterator: 2, 3

# Convert to list if needed
list(enumerate(["a", "b"]))     # [(0, "a"), (1, "b")]
list(zip([1, 2], ["a", "b"]))   # [(1, "a"), (2, "b")]

# Boolean tests (work with any iterable)
any([False, True, False])        # True
all([True, True, True])          # True
all([True, False, True])         # False
```

### Type Method
All objects support the `.type()` method which returns the type name as a string:
```python
x = 42
x.type()                  # "INTEGER"

y = "hello"
y.type()                  # "STRING"

z = [1, 2, 3]
z.type()                  # "LIST"
```

### String Functions
```python
len("hello")                        # 5
upper("hello")                      # "HELLO"
lower("HELLO")                      # "hello"
capitalize("hello world")           # "Hello world"
title("hello world")                # "Hello World"
split("a,b,c", ",")                # ["a", "b", "c"]
join(["a", "b", "c"], "-")         # "a-b-c"
replace("hello world", "world", "python")  # "hello python"
strip("  hello  ")                 # "hello"
lstrip("  hello  ")                # "hello  "
rstrip("  hello  ")                # "  hello"
startswith("hello", "he")          # True
endswith("hello", "lo")            # True
"a" * 3                            # "aaa" (repetition)
3 * "a"                            # "aaa" (repetition)
```

### String Methods (called on string objects)
```python
s = "hello world"
s.find("world")                    # 6 (index of substring, -1 if not found)
s.index("world")                   # 6 (like find, raises error if not found)
s.count("o")                       # 2 (count occurrences)

# String formatting
"Hello, {}!".format("World")       # "Hello, World!"
"{} + {} = {}".format(1, 2, 3)    # "1 + 2 = 3"

# Character type checks
"123".isdigit()                    # True
"abc".isalpha()                    # True
"abc123".isalnum()                 # True
"   ".isspace()                    # True
"HELLO".isupper()                  # True
"hello".islower()                  # True

# Case conversion
"Hello World".swapcase()           # "hELLO wORLD"

# Splitting and partitioning
"hello\nworld".splitlines()        # ["hello", "world"]
"hello-world".partition("-")       # ("hello", "-", "world")
"a-b-c".rpartition("-")            # ("a-b", "-", "c")

# Prefix/suffix removal
"TestCase".removeprefix("Test")    # "Case"
"file.py".removesuffix(".py")      # "file"

# Encoding
"ABC".encode()                     # [65, 66, 67] (byte values)

# Padding and alignment
"42".zfill(5)                      # "00042"
"-42".zfill(5)                     # "-0042"
"hi".center(6)                     # "  hi  "
"hi".center(7, "*")                # "**hi***"
"hi".ljust(5)                      # "hi   "
"hi".rjust(5)                      # "   hi"
```

### Set Methods
```python
s = set([1, 2])
s.add(3)            # s is now {1, 2, 3}
s.remove(2)         # s is now {1, 3}
s.discard(99)       # No error if element not found
s.pop()             # Removes and returns arbitrary element
s.clear()           # Removes all elements
s.copy()            # Returns a shallow copy

# Set operations
s1 = set([1, 2])
s2 = set([2, 3])
s1.union(s2)                # {1, 2, 3}
s1.intersection(s2)         # {2}
s1.difference(s2)           # {1}
s1.symmetric_difference(s2) # {1, 3}
s1.issubset(s2)             # False
s1.issuperset(s2)           # False
```

### List Functions
```python
len([1, 2, 3])                     # 3

# append modifies list in-place (like Python)
my_list = [1, 2]
my_list.append(3)                  # my_list is now [1, 2, 3]
print(my_list)                     # [1, 2, 3]

# extend modifies list in-place by appending elements from another list
list_a = [1, 2]
list_b = [3, 4]
list_a.extend(list_b)              # list_a is now [1, 2, 3, 4]

# sum returns the sum of all numeric elements
sum([1, 2, 3, 4, 5])              # 15
sum([1.5, 2.5, 3.0])              # 7.0
sum((1, 2, 3))                    # 10 (works with tuples too)

# sorted returns a new sorted list (doesn't modify original)
sorted([3, 1, 4, 1, 5])           # [1, 1, 3, 4, 5]
sorted(["banana", "apple"])       # ["apple", "banana"]
sorted([3, 1.5, 2], len)          # Sort with key function
sorted([3, 1, 2], reverse=True)   # [3, 2, 1]
```

### List Methods (called on list objects)
```python
lst = [10, 20, 30, 20, 40]
lst.index(20)                      # 1 (first index of value)
lst.count(20)                      # 2 (count occurrences)

lst = [1, 2, 3, 4, 5]
lst.pop()                          # 5 (removes and returns last element)
lst.pop(0)                         # 1 (removes and returns element at index)

lst = [1, 2, 4, 5]
lst.insert(2, 3)                   # lst is now [1, 2, 3, 4, 5]

lst = [1, 2, 3, 2, 4]
lst.remove(2)                      # lst is now [1, 3, 2, 4] (removes first occurrence)

lst = [1, 2, 3]
lst.clear()                        # lst is now []

original = [1, 2, 3]
copied = original.copy()           # shallow copy

lst = [1, 2, 3, 4, 5]
lst.reverse()                      # lst is now [5, 4, 3, 2, 1]
```

### Range Function
```python
# range() returns an iterator (lazy evaluation)
range(5)                           # Iterator: 0, 1, 2, 3, 4
range(2, 7)                        # Iterator: 2, 3, 4, 5, 6
range(0, 10, 2)                    # Iterator: 0, 2, 4, 6, 8
range(10, 0, -2)                   # Iterator: 10, 8, 6, 4, 2

# Convert to list if needed
list(range(5))                     # [0, 1, 2, 3, 4]

# Use in for loops (iterators work directly)
for i in range(5):
    print(i)
```

### Dictionary Methods
```python
person = {"name": "Alice", "age": "30"}

keys(person)                       # ["name", "age"]
values(person)                     # ["Alice", "30"]
items(person)                      # [["name", "Alice"], ["age", "30"]]

# Iterate over dictionary
for item in items(person):
    key = item[0]
    value = item[1]
    print(key, value)
```

### Dict Methods (called on dict objects)
```python
d = {"a": 1, "b": 2, "c": 3}
d.get("a")                         # 1
d.get("x")                         # None
d.get("x", "default")              # "default"

d = {"a": 1, "b": 2, "c": 3}
d.pop("b")                         # 2 (removes and returns value)
d.pop("x", "not found")            # "not found" (with default)

d1 = {"a": 1, "b": 2}
d2 = {"b": 20, "c": 3}
d1.update(d2)                      # d1 is now {"a": 1, "b": 20, "c": 3}

d = {"a": 1, "b": 2}
d.clear()                          # d is now {}

original = {"a": 1, "b": 2}
copied = original.copy()           # shallow copy

d = {"a": 1}
d.setdefault("a", 100)             # 1 (returns existing value)
d.setdefault("b", 200)             # 200 (sets and returns new value)
```

### Library Import
```python
# Import libraries dynamically. The import statement loads the library
# and makes its functions available as a global object.
import json    # Load JSON library, creates a global 'json' object
import requests    # Load Requests library, creates a global 'requests' object
import re   # Load regex library, creates a global 'regex' object

# Use imported libraries directly via their global object
data = json.parse('{"key":"value"}')
options = {"timeout": 10}
response = requests.get("https://api.example.com", options)
matches = re.findall("[0-9]+", "abc123def456")
```

### JSON Functions
```python
# Parse JSON string to Scriptling objects
data = json.parse('{"name":"Alice","age":30}')
name = data["name"]                # "Alice"
age = data["age"]                  # 30

# Convert Scriptling objects to JSON string
obj = {"name": "Bob", "age": "25"}
json_str = json.stringify(obj)    # '{"age":"25","name":"Bob"}'
```

### HTTP Functions

All HTTP functions return a dictionary with:
- `status`: HTTP status code (integer)
- `body`: Response body (string)
- `headers`: Dictionary of response headers

#### GET Request
```python
# Basic GET (default 5 second timeout)
response = requests.get("https://api.example.com/users")
status = response["status"]        # 200
body = response["body"]            # Response body string
data = json.parse(body)            # Parse JSON response

# GET with options
options = {"timeout": 10}
response = requests.get("https://api.example.com/users", options)

# GET with headers and timeout
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123", "Accept": "application/json"}
}
response = requests.get("https://api.example.com/users", options)
```

#### POST Request
```python
# POST with JSON body (default 5 second timeout)
payload = {"name": "Alice", "email": "alice@example.com"}
body = json.stringify(payload)
response = requests.post("https://api.example.com/users", body)

# POST with options
options = {"timeout": 15}
response = requests.post("https://api.example.com/users", body, options)

# POST with headers and timeout
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123", "Content-Type": "application/json"}
}
response = requests.post("https://api.example.com/users", body, options)

# Check status
if response["status"] == 201:
    print("Created successfully")
```

#### PUT Request
```python
# Update resource (default 5 second timeout)
payload = {"name": "Alice Updated"}
body = json.stringify(payload)
response = requests.put("https://api.example.com/users/1", body)

# With options
options = {"timeout": 10}
response = requests.put("https://api.example.com/users/1", body, options)
```

#### DELETE Request
```python
# Delete resource (default 5 second timeout)
response = requests.delete("https://api.example.com/users/1")

# With options
options = {"timeout": 10}
response = requests.delete("https://api.example.com/users/1", options)
```

#### PATCH Request
```python
# Partial update (default 5 second timeout)
payload = {"email": "newemail@example.com"}
body = json.stringify(payload)
response = requests.patch("https://api.example.com/users/1", body)

# With options
options = {"timeout": 10}
response = requests.patch("https://api.example.com/users/1", body, options)
```

#### Timeout Behavior
- Default timeout: 5 seconds
- On timeout: Returns error
- Timeout parameter: Integer (seconds) in options dictionary

## Complete REST API Example

```python
# Fetch user
options = {"timeout": 10}
response = requests.get("https://api.example.com/users/1", options)

if response["status"] == 200:
    user = json.parse(response["body"])
    print("User: " + user["name"])

    # Update user
    user["email"] = "updated@example.com"
    body = json.stringify(user)
    update_resp = requests.put("https://api.example.com/users/1", body, options)

    if update_resp["status"] == 200:
        print("Updated successfully")
    else:
        print("Update failed: " + str(update_resp["status"]))
else:
    print("Failed to fetch user")

# Create new user
new_user = {"name": "Bob", "email": "bob@example.com"}
body = json.stringify(new_user)
create_resp = requests.post("https://api.example.com/users", body, options)

if create_resp["status"] == 201:
    created = json.parse(create_resp["body"])
    user_id = created["id"]
    print("Created user with ID: " + user_id)

    # Delete user
    delete_resp = requests.delete("https://api.example.com/users/" + user_id, options)
    if delete_resp["status"] == 204:
        print("Deleted successfully")
```

## Indexing and Slicing

### Single Index
```python
word = "hello"
first = word[0]    # "h"
last = word[4]     # "o"

numbers = [1, 2, 3, 4, 5]
first_num = numbers[0]    # 1
```

### Slice Notation
```python
# Lists - basic slicing
numbers = [0, 1, 2, 3, 4, 5]
numbers[1:4]       # [1, 2, 3]
numbers[:3]        # [0, 1, 2]
numbers[3:]        # [3, 4, 5]

# Lists - with step
numbers[::2]       # [0, 2, 4] - every second element
numbers[1::2]      # [1, 3, 5] - every second element starting from index 1
numbers[1:8:2]     # [1, 3, 5, 7] - every second element from 1 to 8

# Lists - reverse with negative step
numbers[::-1]      # [5, 4, 3, 2, 1, 0] - reverse the list
numbers[::-2]      # [5, 3, 1] - every second element in reverse
numbers[4:1:-1]    # [4, 3, 2] - reverse from index 4 to 1

# Strings - basic slicing
text = "Hello World"
text[0:5]          # "Hello"
text[6:]           # "World"
text[:5]           # "Hello"

# Strings - with step
text[::2]          # "HloWrd" - every second character
text[::-1]         # "dlroW olleH" - reverse the string
text[1:8:2]        # "el o" - every second character from 1 to 8
```

## Limitations & Differences from Python

### Not Supported
- List comprehensions
- Lambda functions
- Classes and objects
- Global/nonlocal keywords
- Decorators
- Generators
- `with` statement

### Key Differences
- No implicit type coercion in most operations

## Best Practices

### Error Handling
```python
# Check HTTP status codes
response = requests.get("https://api.example.com/data")
if response["status"] != 200:
    print("Error: " + str(response["status"]))
    return

# Validate data before use
data = json_parse(response["body"])
if data["count"] > 0:
    # Process data
```

### Timeouts
```python
# Always specify timeouts for external calls
response = requests.get("https://slow-api.com/data", 5)  # 5 second timeout
```

### JSON Handling
```python
# Always parse JSON responses
response = requests.get("https://api.example.com/users")
users = json_parse(response["body"])

# Always stringify before sending
payload = {"name": "Alice"}
body = json_stringify(payload)
requests.post("https://api.example.com/users", body)
```

### Variable Naming
```python
# Use descriptive names
user_count = 10
api_response = requests.get(url)

# Not: x = 10, r = requests.get(url)
```

## File Extension

Scriptling scripts use the `.py` extension for syntax highlighting in editors:
- `script.py` - Scriptling script file
- Most Python syntax highlighters work well with Scriptling

## Testing

```bash
# Run all tests
go test ./...

# Run specific tests
go test ./evaluator -v
go test -run TestHTTP -v
```

## Examples

See the `examples/` directory:
- `basic.py` - Variables, operators, control flow
- `functions.py` - Function definitions and recursion
- `collections.py` - Lists, dictionaries, for loops
- `rest_api.py` - Complete REST API examples

## Summary for LLMs

When generating Scriptling code:
1. Use 4-space indentation for blocks
2. Use `True`/`False` for booleans, `None` for null (all capitalized)
3. Use `range(n)`, `range(start, stop)`, or `range(start, stop, step)` for numeric loops
4. Use slice notation: `list[1:3]`, `list[:3]`, `list[3:]`, `list[::2]`, `list[::-1]` (step supported)
5. Use `keys(dict)`, `values(dict)`, `items(dict)` for dictionary iteration
6. HTTP functions return `{"status": int, "body": string, "headers": dict}`
7. HTTP functions accept optional options dictionary with `timeout` and `headers` keys
8. Use `import json`, `import requests`, `import re` to load libraries
9. Always use `json.parse()` and `json.stringify()` for JSON (dot notation)
10. Always use `requests.get()`, `requests.post()`, etc. for HTTP (dot notation)
11. Use `re.match()`, `re.search()`, `re.findall()`, `re.sub()`, `re.split()` for regex (Python-compatible)
12. Default HTTP timeout is 5 seconds if not specified
13. Use `elif` for multiple conditions
14. Use augmented assignment: `x += 1`, `x *= 2`, etc.
15. Use `break` to exit loops, `continue` to skip iterations
16. Use `pass` as a placeholder in empty blocks
17. `append(list, item)` modifies list in-place (like Python)
18. Strings use `+` for concatenation
19. Use `.py` file extension
20. Check `response["status"]` before processing
21. Use `try`/`except`/`finally` for error handling
22. Use `raise "message"` to raise custom errors
23. Multiple assignment: `a, b = [1, 2]` for unpacking lists

## Quick Syntax Reference

```python
# Variables
x = 10

# Augmented assignment
x += 5
x *= 2

# Booleans and None
flag = True
done = False
result = None

# Control flow
if x > 10:
    print("large")
elif x > 5:
    print("medium")
else:
    print("small")

while x > 0:
    x -= 1

for item in [1, 2, 3]:
    if item == 2:
        continue  # Skip 2
    print(item)

# break exits loop
for item in [1, 2, 3, 4, 5]:
    if item == 4:
        break  # Stop at 4
    print(item)

# Functions
def add(a, b):
    return a + b

# Error handling
try:
    result = risky_operation()
except:
    result = None
finally:
    cleanup()

# Raise errors
if x < 0:
    raise "Invalid value"

# Multiple assignment
a, b = [1, 2]
x, y = [y, x]  # Swap

# Lists & Dicts
nums = [1, 2, 3]
data = {"key": "value"}
first = nums[0]
val = data["key"]

# Range and slicing
for i in range(5):
    print(i)

sublist = nums[1:3]  # [2, 3]
text = "hello"[1:4]  # "ell"

# Dictionary methods
for item in items(data):
    print(item[0], item[1])

# HTTP with headers and status check
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token"}
}
resp = requests.get("https://api.example.com/data", options)
if resp["status"] == 200:
    data = json.parse(resp["body"])
```

## Classes

Scriptling supports defining classes with methods and instance fields.

### Class Definition

```python
class Person:
    def __init__(self, name, age):
        self.name = name
        self.age = age

    def greet(self):
        return "Hello, my name is " + self.name

    def is_adult(self):
        return self.age >= 18
```

### Instantiation and Usage

```python
# Create an instance
p = Person("Alice", 30)

# Access fields
print(p.name)  # "Alice"

# Call methods
print(p.greet())  # "Hello, my name is Alice"

# Modify fields
p.age = 31
```

### Key Concepts

1.  **`class` Keyword**: Used to define a new class.
2.  **`__init__` Method**: The constructor method, called when a new instance is created.
3.  **`self` Parameter**: The first parameter of every method is a reference to the current instance (similar to `this` in other languages, but explicit like in Python).
4.  **Fields**: Instance variables are accessed and assigned using `self.field_name`.

### Inheritance

Scriptling supports single inheritance. A class can inherit from another class by specifying the parent class in parentheses after the class name.

```python
class Animal:
    def __init__(self, name):
        self.name = name

    def speak(self):
        return "Generic sound"

class Dog(Animal):
    def __init__(self, name, breed):
        # Call parent constructor
        super(Dog, self).__init__(name)
        self.breed = breed

    def speak(self):
        # Call parent method
        return super(Dog, self).speak() + " and Woof!"

d = Dog("Buddy", "Pug")
print(d.speak())  # "Generic sound and Woof!"
```

### The `super()` Function

The `super()` function returns a proxy object that delegates method calls to a parent or sibling class. This is useful for accessing inherited methods that have been overridden in a class.

- **Syntax**:
    - `super()`: Parameterless version (Python 3 style). Automatically infers the class and instance from the context. Requires the first argument of the method to be named `self`.
    - `super(CurrentClass, self)`: Explicit version.

- **Arguments**:
    - `CurrentClass`: The class object where the method is defined.
    - `self`: The instance object.

**Note**: The parameterless `super()` only works inside class methods where the first argument is named `self`.

### Limitations

- **Nested Classes**: Defining classes inside other classes or functions is **not** supported. Classes must be defined at the top level of a module.
- **Multiple Inheritance**: Only single inheritance is supported.
