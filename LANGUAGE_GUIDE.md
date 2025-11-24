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

### Case Sensitivity
- Keywords: lowercase (`if`, `while`, `def`, `return`)
- Booleans: `True`, `False` (capitalized)
- Variables: case-sensitive (`myVar` ≠ `myvar`)

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

### None/Null
Represented as `None` (not directly creatable, returned by functions with no return value)

## Operators

### Arithmetic
```python
x + y    # Addition
x - y    # Subtraction
x * y    # Multiplication
x / y    # Division (integer division for ints)
x % y    # Modulo
```

### Augmented Assignment
```python
x += y   # x = x + y
x -= y   # x = x - y
x *= y   # x = x * y
x /= y   # x = x / y
x %= y   # x = x % y
```

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
x and y  # Logical AND
x or y   # Logical OR
not x    # Logical NOT
```

### Precedence (highest to lowest)
1. Parentheses `()`
2. Function calls, indexing `func()`, `list[0]`
3. Unary `-`, `not`
4. `*`, `/`, `%`
5. `+`, `-`
6. `<`, `>`, `<=`, `>=`
7. `==`, `!=`
8. `and`
9. `or`

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
```

### Type Conversions
```python
str(42)                   # "42"
int("42")                 # 42
int(3.14)                 # 3
float("3.14")             # 3.14
float(42)                 # 42.0
```

### String Functions
```python
len("hello")                        # 5
upper("hello")                      # "HELLO"
lower("HELLO")                      # "hello"
split("a,b,c", ",")                # ["a", "b", "c"]
join(["a", "b", "c"], "-")         # "a-b-c"
replace("hello world", "world", "python")  # "hello python"
```

### List Functions
```python
len([1, 2, 3])                     # 3

# append modifies list in-place (like Python)
my_list = [1, 2]
append(my_list, 3)                 # my_list is now [1, 2, 3]
print(my_list)                     # [1, 2, 3]
```

### Range Function
```python
range(5)                           # [0, 1, 2, 3, 4]
range(2, 7)                        # [2, 3, 4, 5, 6]
range(0, 10, 2)                    # [0, 2, 4, 6, 8]
range(10, 0, -2)                   # [10, 8, 6, 4, 2]

# Use in for loops
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

### Library Import
```python
# Import libraries dynamically
import("json")    # Load JSON library
import("http")    # Load HTTP library

# Use imported libraries
data = json.parse('{"key":"value"}')
response = http.get("https://api.example.com", 10)
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
response = http.get("https://api.example.com/users")
status = response["status"]        # 200
body = response["body"]            # Response body string
data = json.parse(body)            # Parse JSON response

# GET with options
options = {"timeout": 10}
response = http.get("https://api.example.com/users", options)

# GET with headers and timeout
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123", "Accept": "application/json"}
}
response = http.get("https://api.example.com/users", options)
```

#### POST Request
```python
# POST with JSON body (default 5 second timeout)
payload = {"name": "Alice", "email": "alice@example.com"}
body = json.stringify(payload)
response = http.post("https://api.example.com/users", body)

# POST with options
options = {"timeout": 15}
response = http.post("https://api.example.com/users", body, options)

# POST with headers and timeout
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123", "Content-Type": "application/json"}
}
response = http.post("https://api.example.com/users", body, options)

# Check status
if response["status"] == 201:
    print("Created successfully")
```

#### PUT Request
```python
# Update resource (default 5 second timeout)
payload = {"name": "Alice Updated"}
body = json.stringify(payload)
response = http.put("https://api.example.com/users/1", body)

# With options
options = {"timeout": 10}
response = http.put("https://api.example.com/users/1", body, options)
```

#### DELETE Request
```python
# Delete resource (default 5 second timeout)
response = http.delete("https://api.example.com/users/1")

# With options
options = {"timeout": 10}
response = http.delete("https://api.example.com/users/1", options)
```

#### PATCH Request
```python
# Partial update (default 5 second timeout)
payload = {"email": "newemail@example.com"}
body = json.stringify(payload)
response = http.patch("https://api.example.com/users/1", body)

# With options
options = {"timeout": 10}
response = http.patch("https://api.example.com/users/1", body, options)
```

#### Timeout Behavior
- Default timeout: 5 seconds
- On timeout: Returns error
- Timeout parameter: Integer (seconds) in options dictionary

## Complete REST API Example

```python
# Fetch user
options = {"timeout": 10}
response = http.get("https://api.example.com/users/1", options)

if response["status"] == 200:
    user = json.parse(response["body"])
    print("User: " + user["name"])

    # Update user
    user["email"] = "updated@example.com"
    body = json.stringify(user)
    update_resp = http.put("https://api.example.com/users/1", body, options)

    if update_resp["status"] == 200:
        print("Updated successfully")
    else:
        print("Update failed: " + str(update_resp["status"]))
else:
    print("Failed to fetch user")

# Create new user
new_user = {"name": "Bob", "email": "bob@example.com"}
body = json.stringify(new_user)
create_resp = http.post("https://api.example.com/users", body, options)

if create_resp["status"] == 201:
    created = json.parse(create_resp["body"])
    user_id = created["id"]
    print("Created user with ID: " + user_id)

    # Delete user
    delete_resp = http.delete("https://api.example.com/users/" + user_id, options)
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
# Lists
numbers = [0, 1, 2, 3, 4, 5]
numbers[1:4]       # [1, 2, 3]
numbers[:3]        # [0, 1, 2]
numbers[3:]        # [3, 4, 5]

# Strings
text = "Hello World"
text[0:5]          # "Hello"
text[6:]           # "World"
text[:5]           # "Hello"
```

## Limitations & Differences from Python

### Not Supported
- List comprehensions
- Lambda functions
- Classes and objects
- Exception handling (try/except)
- Multiple assignment: `a, b = 1, 2`
- Global/nonlocal keywords
- Decorators
- Generators
- `with` statement

### Key Differences
- No `None` literal (use functions that return None)
- String concatenation: `+` only (no f-strings or %)
- No implicit type coercion in most operations

## Best Practices

### Error Handling
```python
# Check HTTP status codes
response = http_get("https://api.example.com/data")
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
response = http_get("https://slow-api.com/data", 5)  # 5 second timeout
```

### JSON Handling
```python
# Always parse JSON responses
response = http_get("https://api.example.com/users")
users = json_parse(response["body"])

# Always stringify before sending
payload = {"name": "Alice"}
body = json_stringify(payload)
http_post("https://api.example.com/users", body)
```

### Variable Naming
```python
# Use descriptive names
user_count = 10
api_response = http_get(url)

# Not: x = 10, r = http_get(url)
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
2. Use `True`/`False` for booleans (capitalized)
3. Use `range(n)`, `range(start, stop)`, or `range(start, stop, step)` for numeric loops
4. Use slice notation: `list[1:3]`, `list[:3]`, `list[3:]`, `string[0:5]`
5. Use `keys(dict)`, `values(dict)`, `items(dict)` for dictionary iteration
6. HTTP functions return `{"status": int, "body": string, "headers": dict}`
7. HTTP functions accept optional options dictionary with `timeout` and `headers` keys
8. Use `import("json")` and `import("http")` to load libraries
9. Always use `json.parse()` and `json.stringify()` for JSON (dot notation)
10. Always use `http.get()`, `http.post()`, etc. for HTTP (dot notation)
11. Default HTTP timeout is 5 seconds if not specified
12. Use `elif` for multiple conditions
13. Use augmented assignment: `x += 1`, `x *= 2`, etc.
14. Use `break` to exit loops, `continue` to skip iterations
15. Use `pass` as a placeholder in empty blocks
16. `append(list, item)` modifies list in-place (like Python)
17. Strings use `+` for concatenation
18. Use `.py` file extension
19. Check `response["status"]` before processing

## Quick Syntax Reference

```python
# Variables
x = 10

# Augmented assignment
x += 5
x *= 2

# Booleans
flag = True
done = False

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
resp = http.get("https://api.example.com/data", options)
if resp["status"] == 200:
    data = json.parse(resp["body"])
```
