# Scriptling Guide for LLMs

This guide provides a quick reference for Large Language Models generating Scriptling code. Scriptling is a Python-like scripting language designed for embedding in Go applications.

## Quick Summary for Code Generation

When generating Scriptling code:

1. **Indentation**: Use 4-space indentation for blocks
2. **Booleans and Null**: Use `True`/`False` for booleans, `None` for null (all capitalized)
3. **Loops**: Use `range(n)`, `range(start, stop)`, or `range(start, stop, step)` for numeric loops
4. **Slicing**: Use slice notation: `list[1:3]`, `list[:3]`, `list[3:]`, `list[::2]`, `list[::-1]` (step supported)
5. **Dict Iteration**: Use `keys(dict)`, `values(dict)`, `items(dict)` for dictionary iteration
6. **HTTP Response**: HTTP functions return a Response object with `status_code`, `body`, `text`, `headers`, `url` fields
7. **HTTP Options**: HTTP functions accept optional options dictionary with `timeout` and `headers` keys
8. **Import Libraries**: Use `import json`, `import requests`, `import re` to load libraries
9. **JSON Functions**: Always use `json.loads()` and `json.dumps()` for JSON (dot notation)
10. **HTTP Functions**: Always use `requests.get()`, `requests.post()`, etc. for HTTP (dot notation)
11. **Regex**: Use `re.match()`, `re.search()`, `re.findall()`, `re.sub()`, `re.split()` for regex
12. **Timeouts**: Default HTTP timeout is 5 seconds if not specified
13. **Conditions**: Use `elif` for multiple conditions
14. **Augmented Assignment**: Use augmented assignment: `x += 1`, `x *= 2`, etc.
15. **Loop Control**: Use `break` to exit loops, `continue` to skip iterations
16. **Placeholder**: Use `pass` as a placeholder in empty blocks
17. **List Append**: `append(list, item)` modifies list in-place (like Python)
18. **String Concat**: Strings use `+` for concatenation
19. **File Extension**: Use `.py` file extension
20. **Status Checks**: Check `response.status_code` before processing
21. **Error Handling**: Use `try`/`except`/`finally` for error handling
22. **Raise Errors**: Use `raise "message"` to raise custom errors
23. **Unpacking**: Multiple assignment: `a, b = [1, 2]` for unpacking lists
24. **Variadic Args**: Use `*args` to collect extra positional arguments into a list
25. **Keyword Args**: Use `**kwargs` to collect extra keyword arguments into a dictionary

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

# Variadic arguments (*args)
def sum_all(*args):
    total = 0
    for num in args:
        total += num
    return total

# Keyword arguments collection (**kwargs)
def test_kwargs(**kwargs):
    return kwargs

result = test_kwargs(a=1, b=2)  # {"a": 1, "b": 2}

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
if resp.status_code == 200:
    data = json.loads(resp.body)
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

The `super()` function returns a proxy object that delegates method calls to a parent or sibling class. This is useful for accessing inherited methods that have been overridden in a class.

- **Syntax**:
  - `super()`: Parameterless version (Python 3 style). Automatically infers the class and instance from the context. Requires the first argument of the method to be named `self`.
  - `super(CurrentClass, self)`: Explicit version.

**Note**: The parameterless `super()` only works inside class methods where the first argument is named `self`.

## Key Differences from Python

- **No Nested Classes**: Classes must be defined at the top level of a module
- **Single Inheritance Only**: Multiple inheritance is not supported
- **HTTP Response Format**: Response object with `status_code`, `body`, `text`, `headers`, `url` fields
- **Default Timeout**: HTTP requests have a 5-second default timeout

## See Also

- [LANGUAGE_GUIDE.md](LANGUAGE_GUIDE.md) - Complete language reference
- [QUICK_REFERENCE.md](QUICK_REFERENCE.md) - Library recipes and common patterns
- [LIBRARIES.md](LIBRARIES.md) - Available libraries overview
