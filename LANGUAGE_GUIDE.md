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
append(numbers, 6)        # Returns new list: [1, 2, 3, 4, 5, 6]
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
Not directly supported. Access keys individually.

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
append([1, 2], 3)                  # [1, 2, 3] (returns new list)
```

### JSON Functions
```python
# Parse JSON string to Scriptling objects
data = json_parse('{"name":"Alice","age":30}')
name = data["name"]                # "Alice"
age = data["age"]                  # 30

# Convert Scriptling objects to JSON string
obj = {"name": "Bob", "age": "25"}
json_str = json_stringify(obj)    # '{"age":"25","name":"Bob"}'
```

### HTTP Functions

All HTTP functions return a dictionary with:
- `status`: HTTP status code (integer)
- `body`: Response body (string)
- `headers`: Dictionary of response headers

#### GET Request
```python
# Basic GET
response = http_get("https://api.example.com/users")
status = response["status"]        # 200
body = response["body"]            # Response body string
data = json_parse(body)            # Parse JSON response

# GET with timeout (seconds)
response = http_get("https://api.example.com/users", 10)

# GET with headers
headers = {"Authorization": "Bearer token123", "Accept": "application/json"}
response = http_get("https://api.example.com/users", headers, 10)

# GET with headers and timeout (order flexible)
response = http_get("https://api.example.com/users", 10, headers)
```

#### POST Request
```python
# POST with JSON body
payload = {"name": "Alice", "email": "alice@example.com"}
body = json_stringify(payload)
response = http_post("https://api.example.com/users", body)

# POST with timeout
response = http_post("https://api.example.com/users", body, 15)

# POST with headers
headers = {"Authorization": "Bearer token123", "Content-Type": "application/json"}
response = http_post("https://api.example.com/users", body, headers)

# POST with headers and timeout (order flexible)
response = http_post("https://api.example.com/users", body, headers, 10)

# Check status
if response["status"] == 201:
    print("Created successfully")
```

#### PUT Request
```python
# Update resource
payload = {"name": "Alice Updated"}
body = json_stringify(payload)
response = http_put("https://api.example.com/users/1", body)

# With timeout
response = http_put("https://api.example.com/users/1", body, 10)
```

#### DELETE Request
```python
# Delete resource
response = http_delete("https://api.example.com/users/1")

# With timeout
response = http_delete("https://api.example.com/users/1", 10)
```

#### PATCH Request
```python
# Partial update
payload = {"email": "newemail@example.com"}
body = json_stringify(payload)
response = http_patch("https://api.example.com/users/1", body)

# With timeout
response = http_patch("https://api.example.com/users/1", body, 10)
```

#### Timeout Behavior
- Default timeout: 30 seconds
- On timeout: Returns error
- Timeout parameter: Integer (seconds)

## Complete REST API Example

```python
# Fetch user
response = http_get("https://api.example.com/users/1", 10)

if response["status"] == 200:
    user = json_parse(response["body"])
    print("User: " + user["name"])

    # Update user
    user["email"] = "updated@example.com"
    body = json_stringify(user)
    update_resp = http_put("https://api.example.com/users/1", body, 10)

    if update_resp["status"] == 200:
        print("Updated successfully")
    else:
        print("Update failed: " + str(update_resp["status"]))
else:
    print("Failed to fetch user")

# Create new user
new_user = {"name": "Bob", "email": "bob@example.com"}
body = json_stringify(new_user)
create_resp = http_post("https://api.example.com/users", body, 10)

if create_resp["status"] == 201:
    created = json_parse(create_resp["body"])
    user_id = created["id"]
    print("Created user with ID: " + user_id)

    # Delete user
    delete_resp = http_delete("https://api.example.com/users/" + user_id, 10)
    if delete_resp["status"] == 204:
        print("Deleted successfully")
```

## String Indexing

```python
word = "hello"
first = word[0]    # "h"
last = word[4]     # "o"
```

## Limitations & Differences from Python

### Not Supported
- List comprehensions
- Lambda functions
- Classes and objects
- Modules/imports
- Exception handling (try/except)
- Multiple assignment: `a, b = 1, 2`
- Slice notation: `list[1:3]`
- `range()` function
- Dictionary methods: `keys()`, `values()`, `items()`
- `break` and `continue` in loops
- Global/nonlocal keywords
- Decorators
- Generators
- `with` statement
- `pass` statement

### Key Differences
- No `None` literal (use functions that return None)
- String concatenation: `+` only (no f-strings or %)
- Integer division: `/` performs integer division for integers
- No implicit type coercion in most operations
- `append()` returns new list (doesn't modify in place)

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
3. HTTP functions return `{"status": int, "body": string, "headers": dict}`
4. HTTP functions accept optional headers dictionary
5. Always use `json_parse()` and `json_stringify()` for JSON
6. Use `elif` for multiple conditions
7. Use augmented assignment: `x += 1`, `x *= 2`, etc.
8. `append()` returns new list
9. Strings use `+` for concatenation
10. Use `.py` file extension
11. Always specify timeouts for HTTP calls
12. Check `response["status"]` before processing

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
    print(item)

# Functions
def add(a, b):
    return a + b

# Lists & Dicts
nums = [1, 2, 3]
data = {"key": "value"}
first = nums[0]
val = data["key"]

# HTTP with headers and status check
headers = {"Authorization": "Bearer token"}
resp = http_get("https://api.example.com/data", headers, 10)
if resp["status"] == 200:
    data = json_parse(resp["body"])
```
