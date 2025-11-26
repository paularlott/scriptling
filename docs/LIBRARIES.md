# Scriptling Libraries

Scriptling provides both core functions (always available) and optional libraries (loaded on demand).

## Core Functions

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
- `capitalize(string)` - Capitalize first letter
- `title(string)` - Title case
- `split(string, sep)` - Split to list
- `join(list, sep)` - Join from list
- `replace(str, old, new)` - Replace substring
- `strip(string)` - Trim whitespace from both ends
- `lstrip(string)` - Trim whitespace from left
- `rstrip(string)` - Trim whitespace from right
- `startswith(string, prefix)` - Check if string starts with prefix
- `endswith(string, suffix)` - Check if string ends with suffix

### List Functions
- `len(list)` - Get length
- `append(list, item)` - Append item (modifies list in-place)
- `extend(list, other_list)` - Append elements from another list (modifies list in-place)

### System
- `import library_name` - Load library dynamically

## Optional Libraries

These libraries must be imported before use:

### Datetime Library
Datetime functions for formatting and parsing dates and times.

```python
import datetime

now = datetime.now()              # "2025-11-26 11:34:15"
utc = datetime.utcnow()           # "2025-11-26 03:34:15"
today = datetime.today()          # "2025-11-26"
iso = datetime.isoformat()        # "2025-11-26T11:34:15Z"
future = datetime.add_days(now_timestamp, 7)  # Add 7 days
formatted = datetime.now("%Y-%m-%d %H:%M:%S")  # Custom format
```

[See datetime.md](libraries/datetime.md) for complete documentation.

### Math Library
Mathematical functions and constants.

```python
import math

result = math.sqrt(16)      # 4.0
power = math.pow(2, 8)      # 256.0
pi = math.pi()              # 3.14159...
```

[See math.md](libraries/math.md) for complete documentation.

### Time Library
Time-related functions for timestamps and formatting.

```python
import time

now = time.time()           # Current timestamp
formatted = time.strftime("%Y-%m-%d %H:%M:%S", now)
time.sleep(1)               # Sleep 1 second
```

[See time.md](libraries/time.md) for complete documentation.

### JSON Library
Parse and generate JSON data.

```python
import json

data = json.loads('{"name":"Alice"}')
json_str = json.dumps({"key": "value"})
```

[See json.md](libraries/json.md) for complete documentation.

### Regex Library
Regular expression pattern matching and text processing.

```python
import re

matches = re.findall("[0-9]+", "abc123def456")
result = re.replace("[0-9]+", "XXX", "Price: 100")
escaped = re.escape("a.b+c")
full_match = re.fullmatch("[0-9]+", "123")
```

[See regex.md](libraries/regex.md) for complete documentation.

### HTTP Library
Make HTTP requests (requires manual registration).

```go
// In Go code
p.RegisterLibrary("http", extlibs.HTTPLibrary())
```

```python
import http

response = http.get("https://api.example.com/data")
if response.status_code == 200:
    data = response.json()
```

[See http.md](libraries/http.md) for complete documentation.

### Base64 Library
Base64 encoding and decoding.

```python
import base64

encoded = base64.encode("hello")
decoded = base64.decode(encoded)
```

[See base64.md](libraries/base64.md) for complete documentation.

### Hashlib Library
Cryptographic hash functions.

```python
import hashlib

md5 = hashlib.md5("data")
sha256 = hashlib.sha256("data")
```

[See hashlib.md](libraries/hashlib.md) for complete documentation.

### Random Library
Random number generation.

```python
import random

num = random.randint(1, 100)
choice = random.choice(["a", "b", "c"])
```

[See random.md](libraries/random.md) for complete documentation.

### URL Library
URL parsing, encoding, and manipulation.

```python
import lib

parsed = lib.urlparse("https://example.com/path?query=value")
encoded = lib.quote("hello world")
parts = lib.urlsplit("https://example.com/path?query=value#fragment")
query_dict = lib.parse_qs("key=value1&key=value2")
query_str = lib.urlencode({"key": "value", "list": ["a", "b"]})
```

[See url.md](libraries/url.md) for complete documentation.

## Loading Libraries

### Automatic Loading

```python
# Import loads the library automatically
import json
import math

# Now use them
data = json.parse('{"key": "value"}')
result = math.sqrt(16)
```

### Manual Registration (HTTP)

Some libraries require manual registration in Go:

```go
import "github.com/paularlott/scriptling/extlibs"

p := scriptling.New()
p.RegisterLibrary("http", extlibs.HTTPLibrary())
```

## Creating Custom Libraries

To create custom libraries, see [EXTENDING_SCRIPTLING.md](EXTENDING_SCRIPTLING.md).

## Performance Notes

- Core functions: Zero overhead, always available
- Optional libraries: Loaded on first import, cached thereafter
- HTTP library: Requires explicit registration for security

# Find all numbers
numbers = re.findall("[0-9]+", "abc123def456")

# Replace digits
text = re.replace("[0-9]+", "Price: 100", "XXX")
```

### http

**Functions:**
All return a response object with Python requests-compatible interface:

- `requests.get(url, options?)` - GET request
- `requests.post(url, body, options?)` - POST request
- `requests.put(url, body, options?)` - PUT request
- `requests.delete(url, options?)` - DELETE request
- `requests.patch(url, body, options?)` - PATCH request

**Response Object:**
Python requests-compatible response object with both dictionary and attribute access:

Attributes/Keys:
- `response.status_code` or `response["status_code"]` - HTTP status code (integer)
- `response.text` or `response["text"]` - Response body (string)
- `response["headers"]` - Response headers (dictionary)

Response methods:
- `response.json()` - Parse response body as JSON and return Scriptling object
- `response.raise_for_status()` - Raise error if status code >= 400 (4xx or 5xx)

**Features:**
- HTTP/2 support with automatic fallback to HTTP/1.1
- Connection pooling (100 connections per host)
- Accepts self-signed certificates
- Default timeout: 5 seconds
- Python requests-compatible API for LLM code generation
- Full method support: `json()`, `raise_for_status()`

**Options dictionary:**
- `timeout` (integer): Request timeout in seconds (default: 5)
- `headers` (dictionary): HTTP headers to send

**Example:**
```python
import requests

# Simple GET request (5 second timeout)
response = requests.get("https://api.example.com/users/1")
if response.status_code == 200:
    # Use json() method to parse response
    user = response.json()
    print(user["name"])

# Using raise_for_status() for error handling
try:
    response = requests.get("https://api.example.com/data")
    response.raise_for_status()  # Raises error if 4xx or 5xx
    data = response.json()
    print(data)
except Exception as e:
    print("Request failed:", e)

# Using requests-compatible attributes
response = requests.get("https://api.example.com/data")
if response.status_code == 200:
    content = response.text[:500]  # First 500 chars
    print(content)

# GET with options
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123"}
}
response = requests.get("https://api.example.com/users/1", options)

# POST request with error handling
try:
    new_user = {"name": "Alice", "email": "alice@example.com"}
    # Note: For now, stringify the body manually
    import json
    body = json.stringify(new_user)

    options = {"timeout": 15}
    response = requests.post("https://api.example.com/users", body, options)
    response.raise_for_status()

    created = response.json()
    print("Created user:", created["id"])
except Exception as e:
    print("Error:", e)

# Other methods
response = requests.put(url, body, options)
response = requests.delete(url, options)
response = requests.patch(url, body, options)
```

**Exception Handling:**
The parser supports Python requests-style exception handling with dotted names:

```python
import requests

# LLM-compatible exception handling
try:
    response = requests.get('https://api.example.com/data')
    response.raise_for_status()
    data = response.json()
except requests.exceptions.RequestException as e:
    print(f"Request error: {e}")

# Also supports direct exception names
try:
    response = requests.get('https://api.example.com/data')
    response.raise_for_status()
except requests.HTTPError as e:
    print(f"HTTP error: {e}")
except requests.RequestException as e:
    print(f"Request error: {e}")
```

Note: Currently, all exceptions are caught regardless of the specific exception type specified. Full exception type matching will be implemented in a future version.

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
- `capitalize(string)` - Capitalize first letter
- `title(string)` - Title case
- `split(string, sep)` - Split to list
- `join(list, sep)` - Join from list
- `replace(str, old, new)` - Replace substring
- `strip(string)` - Trim whitespace from both ends
- `lstrip(string)` - Trim whitespace from left
- `rstrip(string)` - Trim whitespace from right
- `startswith(string, prefix)` - Check if string starts with prefix
- `endswith(string, suffix)` - Check if string ends with suffix

### List Functions
- `len(list)` - Get length
- `append(list, item)` - Append item (modifies list in-place)
- `extend(list, other_list)` - Append elements from another list (modifies list in-place)

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
    response = requests.get("https://api.example.com/data", 10)
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
    "time": stdlib.GetTimeLibrary,  // Add here
    "mylib": stdlib.MyLibrary,
}
```

3. Use it:
```python
import mylib
mylib.func1()
```

## Best Practices

1. **Load only what you need**: `scriptling.New()` makes only the core libraries available
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
