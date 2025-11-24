# Scriptling Library System

## Overview

Scriptling uses an optional library system for JSON and HTTP functionality. This keeps the core interpreter lightweight and fast.

## Design

- **Core builtins**: Essential functions always available (`print`, `len`, `str`, `int`, `float`, string operations)
- **Optional libraries**: JSON and HTTP loaded on demand
- **Custom libraries**: Easy to create and register your own

## Loading Libraries

### From Go

```go
// Option 1: Load at creation
p := scriptling.New("json", "http")

// Option 2: No libraries (lightweight)
p := scriptling.New()

// Option 3: Load specific library
p := scriptling.New("json")
```

### From Scriptling Scripts

```python
# Import libraries dynamically
import json
import http

# Now use them
data = json.parse('{"name":"Alice"}')
response = http.get("https://api.example.com/data", 10)
```

## Available Libraries

### json

**Functions:**
- `json.parse(string)` - Parse JSON string to Scriptling objects
- `json.stringify(object)` - Convert Scriptling objects to JSON string

**Example:**
```python
import json

# Parse
data = json.parse('{"users":[{"name":"Alice"},{"name":"Bob"}]}')
first_user = data["users"][0]["name"]  # "Alice"

# Stringify
obj = {"status": "success", "count": "42"}
json_str = json.stringify(obj)  # '{"count":"42","status":"success"}'
```

### regex

**Functions:**
- `re.match(pattern, text)` - Returns True if pattern matches text
- `re.find(pattern, text)` - Returns first match or None
- `re.findall(pattern, text)` - Returns list of all matches
- `re.replace(pattern, text, replacement)` - Replace all matches
- `re.split(pattern, text)` - Split text by pattern

**Example:**
```python
import re

# Match
if re.match("[0-9]+", "abc123"):
    print("Contains digits")

# Find
email = re.find("[a-z]+@[a-z]+\.[a-z]+", "Contact: user@example.com")
print(email)  # "user@example.com"

# Find all
phones = re.findall("[0-9]{3}-[0-9]{4}", "Call 555-1234 or 555-5678")
print(phones)  # ["555-1234", "555-5678"]

# Replace
text = re.replace("[0-9]+", "Price: 100", "REDACTED")
print(text)  # "Price: REDACTED"

# Split
parts = re.split("[,;]", "one,two;three")
print(parts)  # ["one", "two", "three"]
```

### regex

**Functions:**
- `re.match(pattern, text)` - Check if pattern matches (returns boolean)
- `re.find(pattern, text)` - Find first match (returns string or None)
- `re.findall(pattern, text)` - Find all matches (returns list)
- `re.replace(pattern, text, replacement)` - Replace matches (returns string)
- `re.split(pattern, text)` - Split by pattern (returns list)

**Example:**
```python
import re

# Extract email
email = re.find("[a-z]+@[a-z]+\.[a-z]+", "Contact: user@example.com")

# Find all numbers
numbers = re.findall("[0-9]+", "abc123def456")

# Replace digits
text = re.replace("[0-9]+", "Price: 100", "XXX")
```

### http

**Functions:**
All return `{"status": int, "body": string, "headers": dict}`

- `http.get(url, options?)` - GET request
- `http.post(url, body, options?)` - POST request
- `http.put(url, body, options?)` - PUT request
- `http.delete(url, options?)` - DELETE request
- `http.patch(url, body, options?)` - PATCH request

**Features:**
- HTTP/2 support with automatic fallback to HTTP/1.1
- Connection pooling (100 connections per host)
- Accepts self-signed certificates
- Default timeout: 5 seconds

**Options dictionary:**
- `timeout` (integer): Request timeout in seconds (default: 5)
- `headers` (dictionary): HTTP headers to send

**Example:**
```python
import json
import http

# Simple GET request (5 second timeout)
response = http.get("https://api.example.com/users/1")
if response["status"] == 200:
    user = json.parse(response["body"])
    print(user["name"])

# GET with options
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123"}
}
response = http.get("https://api.example.com/users/1", options)

# POST request
new_user = {"name": "Alice", "email": "alice@example.com"}
body = json.stringify(new_user)
options = {"timeout": 15}
response = http.post("https://api.example.com/users", body, options)

if response["status"] == 201:
    print("Created successfully")
else:
    print("Error: " + str(response["status"]))

# Other methods
response = http.put(url, body, options)
response = http.delete(url, options)
response = http.patch(url, body, options)
```

## Core Builtins

Always available without importing:

### I/O
- `print(value)` - Output to console

### Type Conversions
- `str(value)` - Convert to string
- `int(value)` - Convert to integer
- `float(value)` - Convert to float

### String Functions
- `len(string)` - Get length
- `upper(string)` - Uppercase
- `lower(string)` - Lowercase
- `split(string, sep)` - Split to list
- `join(list, sep)` - Join from list
- `replace(str, old, new)` - Replace substring

### List Functions
- `len(list)` - Get length
- `append(list, item)` - Append item (returns new list)

### System
- `import library_name` - Load library dynamically

## Creating Custom Libraries

### Define Library in Go

```go
package mylib

import "github.com/paularlott/scriptling/object"

func MyLibrary() map[string]*object.Builtin {
    return map[string]*object.Builtin{
        "hello": {
            Fn: func(args ...object.Object) object.Object {
                return &object.String{Value: "Hello from custom lib!"}
            },
        },
        "add": {
            Fn: func(args ...object.Object) object.Object {
                if len(args) != 2 {
                    return &object.Error{Message: "need 2 arguments"}
                }
                a := args[0].(*object.Integer).Value
                b := args[1].(*object.Integer).Value
                return &object.Integer{Value: a + b}
            },
        },
    }
}
```

### Register and Use

```go
p := scriptling.New()
p.RegisterLibrary("mylib", mylib.MyLibrary())

p.Eval(`
result = mylib.add(5, 3)
print(mylib.hello())
`)
```

## Performance Benefits

### Without Libraries
```go
p := scriptling.New()  // Lightweight, no HTTP/JSON overhead
p.Eval("x = 5 + 3")  // Fast execution
```

### With Libraries
```go
p := scriptling.New("json", "http")  // Loads JSON + HTTP
p.Eval(`
    response = http.get("https://api.example.com/data", 10)
    data = json.parse(response["body"])
`)
```

## Library Syntax

Libraries support both dot notation and bracket notation:
```python
# Dot notation (recommended, Python-like)
library_name.function_name(arguments)

# Bracket notation (also works)
library_name["function_name"](arguments)
```

This is similar to Python's module system:
- Python: `json.loads(string)`
- Scriptling: `json.parse(string)` or `json["parse"](string)`

## Adding Libraries to Scriptling

To add a new standard library:

1. Create `stdlib/mylib.go`:
```go
package stdlib

func MyLibrary() map[string]*object.Builtin {
    return map[string]*object.Builtin{
        "func1": { Fn: ... },
        "func2": { Fn: ... },
    }
}
```

2. Register in `scriptling.go`:
```go
var availableLibraries = map[string]func() map[string]*object.Builtin{
    "json": stdlib.JSONLibrary,
    "http": stdlib.HTTPLibrary,
    "mylib": stdlib.MyLibrary,  // Add here
}
```

3. Use it:
```python
import mylib
mylib.func1()
```

## Best Practices

1. **Load only what you need**: `scriptling.New()` for core, `scriptling.New("json")` when needed
2. **Use import in scripts**: Dynamic loading based on script needs
3. **Check HTTP status codes**: Always check `response["status"]` before processing
4. **Set timeouts**: Always specify timeouts for HTTP calls
5. **Handle errors**: Check for errors in Go and validate data in Scriptling

## Summary

- **Core**: Minimal, always available
- **Libraries**: Optional, loaded on demand
- **Custom**: Easy to create and register
- **Performance**: Only pay for what you use
- **Pythonic**: Familiar syntax and patterns
