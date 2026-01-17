# Scriptling Language Quick Reference

A condensed syntax cheat sheet for the Scriptling programming language - a Python-like scripting language for embedding in Go applications.

## Variables and Types

```python
# Variables
x = 10
name = "Alice"
price = 3.14

# Booleans and None
flag = True
done = False
result = None

# Lists and Dictionaries
nums = [1, 2, 3]
data = {"key": "value"}
first = nums[0]
val = data["key"]

# Sets
numbers = set([1, 2, 3])
unique = set([1, 2, 2, 3])  # {1, 2, 3}
empty = set()
```

## Operators

```python
# Arithmetic
+, -, *, /, //, %, **

# Comparison
==, !=, <, >, <=, >=

# Boolean/Logical
and, or, not

# Bitwise
&, |, ^, ~, <<, >>

# Augmented Assignment
+=, -=, *=, /=, //=, %=, &=, |=, ^=, <<=, >>=

# Chained comparisons
1 < x < 10        # Equivalent to: 1 < x and x < 10
```

## Control Flow

```python
# If/Elif/Else
if x > 10:
    print("large")
elif x > 5:
    print("medium")
else:
    print("small")

# While Loop
while x > 0:
    x -= 1

# For Loop
for item in [1, 2, 3]:
    if item == 2:
        continue  # Skip 2
    print(item)

# break exits loop
for item in [1, 2, 3, 4, 5]:
    if item == 4:
        break  # Stop at 4
    print(item)

# Match statement (pattern matching)
match status:
    case 200:
        print("Success")
    case 404:
        print("Not found")
    case _:
        print("Other")

# Match with type checking
match data:
    case int():
        print("Integer")
    case str():
        print("String")

# Match with guards
match value:
    case x if x > 100:
        print("Large")
    case x:
        print("Small")

# Match with dict patterns
match response:
    case {"status": 200, "data": payload}:
        process(payload)
    case {"error": msg}:
        print(msg)
```

## Functions

```python
# Definition
def add(a, b):
    return a + b

# Default parameters
def greet(name, greeting="Hello"):
    return greeting + ", " + name

# Keyword arguments
result = my_func(arg1, arg2, key="value")

# Variadic arguments (*args)
def sum_all(*args):
    total = 0
    for num in args:
        total += num
    return total

# Keyword arguments collection (**kwargs)
def test_kwargs(**kwargs):
    return kwargs

result = test_kwargs(a=1, b=2, c=3)  # {"a": 1, "b": 2, "c": 3}

# Combining all parameter types
def func_with_all(a, b=10, *args, **kwargs):
    print("a:", a)
    print("b:", b)
    print("args:", args)
    print("kwargs:", kwargs)

func_with_all(1, 2, 3, 4, x=5, y=6)
# a: 1, b: 2, args: [3, 4], kwargs: {"x": 5, "y": 6}

# Lambda
square = lambda x: x * 2

# Lambda with sorted
sorted(["ccc", "a", "bb"], key=lambda s: len(s))  # ["a", "bb", "ccc"]
```

## Error Handling

```python
# Try/Except/Finally
try:
    result = risky_operation()
except:
    result = None
finally:
    cleanup()

# Raise errors
if x < 0:
    raise "Invalid value"

# Uncaught raise terminates with error
raise "fatal"  # Error: Uncaught exception: fatal

# Bare raise re-raises current exception
try:
    raise "error"
except Exception as e:
    print("Caught: " + str(e))
    raise  # Re-raise the same exception

# Access exception message with str(e)
try:
    raise "something failed"
except Exception as e:
    print(str(e))  # "something failed"

# Assert
assert x > 0, "x must be positive"
```

## Data Structures

```python
# Multiple assignment
a, b = [1, 2]
x, y = [y, x]  # Swap

# Range and slicing
for i in range(5):
    print(i)  # 0, 1, 2, 3, 4

sublist = nums[1:3]  # [2, 3]
text = "hello"[1:4]  # "ell"
reversed_list = nums[::-1]  # [3, 2, 1]

# Dictionary iteration
for item in items(data):
    print(item[0], item[1])

# List operations
append(my_list, item)  # Modifies in-place
sorted([3, 1, 2])      # Returns new sorted list
```

## HTTP and JSON

```python
import json
import requests

# HTTP with headers and status check
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token"}
}
resp = requests.get("https://api.example.com/data", options)
if resp["status"] == 200:
    data = json.parse(resp["body"])

# POST request
payload = {"name": "Alice"}
body = json.stringify(payload)
response = requests.post("https://api.example.com/users", body)
```

## System Functions

```python
import sys

# Exit with status code (raises SystemExit exception)
sys.exit(0)   # Success
sys.exit(1)   # Error

# Catch sys.exit to prevent termination
try:
    sys.exit(42)
except Exception as e:
    print("Caught: " + str(e))  # "Caught: SystemExit: 42"

# Exit with custom message
sys.exit("Fatal error occurred")
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

The `super()` function returns a proxy object that delegates method calls to a parent or sibling class.

- **Syntax**:
  - `super()`: Parameterless version (Python 3 style). Automatically infers the class and instance. Requires first argument named `self`.
  - `super(CurrentClass, self)`: Explicit version.

### Limitations

- **Nested Classes**: Classes must be defined at the top level of a module
- **Multiple Inheritance**: Only single inheritance is supported

## Key Differences from Python

- No nested classes
- No multiple inheritance
- HTTP response always returns `{"status": int, "body": string, "headers": dict}`
- Default HTTP timeout: 5 seconds

## See Also

- [LANGUAGE_GUIDE.md](LANGUAGE_GUIDE.md) - Complete language reference
- [QUICK_REFERENCE.md](QUICK_REFERENCE.md) - Library recipes and patterns
- [LLM_GUIDE.md](LLM_GUIDE.md) - Quick reference for LLM code generation
