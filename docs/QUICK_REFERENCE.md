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

# POST
import json
body = json.stringify({"name": "Alice"})
response = requests.post("https://api.example.com/users", body)

# Other methods
response = requests.put(url, body)
response = requests.delete(url)
response = requests.patch(url, body)

# LLM-compatible exception handling (dotted names supported)
try:
    response = requests.get(url)
    response.raise_for_status()
    content = response.text[:500]
except requests.exceptions.RequestException as e:
    print(f"Error: {e}")
```

### With Options (timeout and/or headers)

```python
# Just timeout
options = {"timeout": 10}
response = requests.get(url, options)

# Just headers
options = {
    "headers": {
        "Authorization": "Bearer token123",
        "Accept": "application/json"
    }
}
response = requests.get(url, options)

# Both timeout and headers
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123"}
}
response = requests.get(url, options)
response = requests.post(url, body, options)
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
sum([1, 2, 3, 4])                   # 10
sorted([3, 1, 2])                   # [1, 2, 3]
sorted(["banana", "apple"], len)    # Sort with key function

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
