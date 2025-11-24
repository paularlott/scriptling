# Scriptling Quick Reference

## JSON Library

```python
import json

# Parse JSON string to objects
data = json.parse('{"name":"Alice","age":30}')
name = data["name"]  # "Alice"

# Convert objects to JSON string
obj = {"status": "success", "count": 42}
json_str = json.stringify(obj)
```

## Regex Library

```python
import re

# Match - returns boolean
if re.match("[0-9]+", "abc123"):
    print("Has digits")

# Find - returns first match or None
email = re.find("[a-z]+@[a-z]+\.[a-z]+", "user@example.com")

# Find all - returns list
phones = re.findall("[0-9]{3}-[0-9]{4}", "555-1234 or 555-5678")

# Replace - returns modified string
text = re.replace("[0-9]+", "Price: 100", "XXX")

# Split - returns list
parts = re.split("[,;]", "one,two;three")
```

## HTTP Library

All methods return: `{"status": int, "body": string, "headers": dict}`

### Simple Requests (5 second default timeout)

```python
import http

# GET
response = http.get("https://api.example.com/users")

# POST
body = json.stringify({"name": "Alice"})
response = http.post("https://api.example.com/users", body)

# PUT
response = http.put("https://api.example.com/users/1", body)

# DELETE
response = http.delete("https://api.example.com/users/1")

# PATCH
response = http.patch("https://api.example.com/users/1", body)
```

### With Options (timeout and/or headers)

```python
# Just timeout
options = {"timeout": 10}
response = http.get(url, options)

# Just headers
options = {
    "headers": {
        "Authorization": "Bearer token123",
        "Accept": "application/json"
    }
}
response = http.get(url, options)

# Both timeout and headers
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123"}
}
response = http.get(url, options)
response = http.post(url, body, options)
```

### Complete Example

```python
import json
import http

# Configure options
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123"}
}

# GET request
response = http.get("https://api.example.com/users/1", options)

if response["status"] == 200:
    user = json.parse(response["body"])
    print("User: " + user["name"])
    
    # Update user
    user["email"] = "new@example.com"
    body = json.stringify(user)
    
    update = http.put("https://api.example.com/users/1", body, options)
    if update["status"] == 200:
        print("Updated!")
else:
    print("Error: " + str(response["status"]))
```

## Core Built-in Functions

Always available without importing:

```python
# I/O
print("Hello")

# Type conversions
str(42)           # "42"
int("42")         # 42
float("3.14")     # 3.14

# String operations
len("hello")                        # 5
upper("hello")                      # "HELLO"
lower("HELLO")                      # "hello"
split("a,b,c", ",")                # ["a", "b", "c"]
join(["a", "b"], "-")              # "a-b"
replace("hello", "l", "L")         # "heLLo"

# List operations
numbers = [1, 2, 3]
len(numbers)                        # 3
append(numbers, 4)                  # numbers is now [1, 2, 3, 4]

# Range
range(5)                            # [0, 1, 2, 3, 4]
range(2, 5)                         # [2, 3, 4]
range(0, 10, 2)                     # [0, 2, 4, 6, 8]

# Dictionary methods
person = {"name": "Alice", "age": 30}
keys(person)                        # ["name", "age"]
values(person)                      # ["Alice", 30]
items(person)                       # [["name", "Alice"], ["age", 30]]
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
    response = http.get(url, options)
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
x / y    # Division (always returns float, e.g., 5 / 2 = 2.5)
x % y    # Modulo (remainder, e.g., 5 % 2 = 1)

# Augmented assignment
x += 5   # x = x + 5
x -= 3   # x = x - 3
x *= 2   # x = x * 2
x /= 4   # x = x / 4

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
   response = http.get(url, options)
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
