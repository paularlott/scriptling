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
- `help([object])` - Display help information for functions, libraries, and objects

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
ts = datetime.timestamp()         # Current Unix timestamp
formatted = datetime.now("%Y-%m-%d %H:%M:%S")  # Custom format

# Date arithmetic with timedelta
one_day = datetime.timedelta(days=1)        # 86400.0 seconds
tomorrow = ts + datetime.timedelta(days=1)
```

[See datetime.md](libraries/datetime.md) for complete documentation.

### Math Library
Mathematical functions and constants.

```python
import math

result = math.sqrt(16)      # 4.0
power = math.pow(2, 8)      # 256.0
pi = math.pi                # 3.14159...
e = math.e                  # 2.71828...
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

### Pathlib Library
Object-oriented filesystem paths (enabled by default in CLI).

```python
import pathlib

# Create path objects
p = pathlib.Path("/home/user/file.txt")

# Access properties
print(p.name)        # "file.txt"
print(p.stem)        # "file"
print(p.suffix)      # ".txt"
print(p.parent)      # "/home/user"

# Path operations
p2 = p.joinpath("subdir", "newfile.py")
exists = p.exists()
is_file = p.is_file()
is_dir = p.is_dir()

# File operations
p.write_text("Hello, World!")
content = p.read_text()
p.unlink()  # Delete file
```

[See pathlib.md](libraries/pathlib.md) for complete documentation.

### Regex Library
Regular expression pattern matching and text processing.

```python
import re

matches = re.findall("[0-9]+", "abc123def456")
result = re.sub("[0-9]+", "XXX", "Price: 100")  # Python-compatible
escaped = re.escape("a.b+c")
full_match = re.fullmatch("[0-9]+", "123")
```

[See regex.md](libraries/regex.md) for complete documentation.

## Extended Libraries

Extended libraries require manual registration in Go or are enabled by default in the CLI.

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

### Requests Library
Python-compatible HTTP client (enabled by default in CLI).

```python
import requests

response = requests.get("https://api.example.com/data")
response = requests.post(url, json={"key": "value"})
```

### Sys Library
System-specific parameters and functions (CLI only by default).

```python
import sys

print(sys.platform)     # "darwin", "linux", or "win32"
print(sys.argv)         # Command-line arguments
sys.exit(1)             # Exit with code
```

[See sys.md](libraries/sys.md) for complete documentation.

### Secrets Library
Cryptographically strong random numbers (CLI only by default).

```python
import secrets

token = secrets.token_hex(32)
secure_choice = secrets.choice(["a", "b", "c"])
```

[See secrets.md](libraries/secrets.md) for complete documentation.

## Standard Libraries

### Base64 Library
Base64 encoding and decoding.

```python
import base64

encoded = base64.b64encode("hello")
decoded = base64.b64decode(encoded)
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

random.seed(42)               # Reproducible results
num = random.randint(1, 100)
flt = random.uniform(0.0, 1.0)
choice = random.choice(["a", "b", "c"])
sample = random.sample([1,2,3,4,5], 3)
```

[See random.md](libraries/random.md) for complete documentation.

### String Library
String constants for character classification.

```python
import string

letters = string.ascii_letters      # a-zA-Z
digits = string.digits              # 0-9
punct = string.punctuation          # !"#$%&'()*+,-./:;<=>?@[\]^_`{|}~
```

[See string.md](libraries/string.md) for complete documentation.

### UUID Library
UUID generation.

```python
import uuid

id1 = uuid.uuid1()    # Time-based UUID
id4 = uuid.uuid4()    # Random UUID
id7 = uuid.uuid7()    # Time-ordered random UUID
```

[See uuid.md](libraries/uuid.md) for complete documentation.

### HTML Library
HTML escaping and unescaping.

```python
import html

safe = html.escape("<script>alert('xss')</script>")
original = html.unescape("&lt;script&gt;")
```

[See html.md](libraries/html.md) for complete documentation.

### Statistics Library
Statistical functions for numeric data.

```python
import statistics

avg = statistics.mean([1, 2, 3, 4, 5])
med = statistics.median([1, 2, 3])
std = statistics.stdev([2, 4, 4, 4, 5, 5, 7, 9])
```

[See statistics.md](libraries/statistics.md) for complete documentation.

### Functools Library
Higher-order functions.

```python
import functools

def add(x, y):
    return x + y

result = functools.reduce(add, [1, 2, 3, 4, 5])  # 15
```

[See functools.md](libraries/functools.md) for complete documentation.

### Itertools Library
Iterator building blocks for efficient looping.

```python
import itertools

chained = itertools.chain([1, 2], [3, 4])            # [1, 2, 3, 4]
perms = itertools.permutations([1, 2, 3])            # All orderings
combs = itertools.combinations([1, 2, 3], 2)         # All pairs
prod = itertools.product([1, 2], ["a", "b"])         # Cartesian product
batches = itertools.batched([1, 2, 3, 4, 5], 2)      # [[1, 2], [3, 4], [5]]
```

[See itertools.md](libraries/itertools.md) for complete documentation.

### Collections Library
Specialized container datatypes.

```python
import collections

counter = collections.Counter([1, 1, 2, 3, 3, 3])   # {1: 2, 2: 1, 3: 3}
d = collections.deque([1, 2, 3])                     # Double-ended queue
Point = collections.namedtuple("Point", ["x", "y"]) # Named tuple factory
cm = collections.ChainMap(dict1, dict2)             # Merged dict view
```

[See collections.md](libraries/collections.md) for complete documentation.

### Copy Library
Shallow and deep copy operations.

```python
import copy

shallow = copy.copy([1, [2, 3]])     # Shallow copy
deep = copy.deepcopy([1, [2, 3]])    # Deep copy (recursive)
```

[See copy.md](libraries/copy.md) for complete documentation.

### Textwrap Library
Text wrapping and filling utilities.

```python
import textwrap

lines = textwrap.wrap("Long text here...", 40)
indented = textwrap.indent("Hello\nWorld", "  ")
```

[See textwrap.md](libraries/textwrap.md) for complete documentation.

### URL Library (urllib.parse)
URL parsing, encoding, and manipulation. Python-compatible with the `urllib.parse` module.

```python
import urllib.parse

# Python-compatible API
parsed = urllib.parse.urlparse("https://example.com/path?query=value")
encoded = urllib.parse.quote("hello world")
encoded_plus = urllib.parse.quote_plus("hello world")  # spaces as +
parts = urllib.parse.urlsplit("https://example.com/path?query=value#fragment")
query_dict = urllib.parse.parse_qs("key=value1&key=value2")
query_str = urllib.parse.urlencode({"key": "value", "list": ["a", "b"]})
```

[See urllib.md](libraries/urllib.md) for complete documentation.

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

## Creating Custom Libraries with Scriptling

You can also create libraries using Scriptling code itself, without writing any Go code.

### Register Single Functions

```go
p := scriptling.New()

// Register a function written in Scriptling
err := p.RegisterScriptFunc("greet", `
def greet(name):
    return "Hello, " + name + "!"
`)
if err != nil {
    fmt.Println("Error:", err)
    return
}

// Use the registered function
p.Eval(`print(greet("World"))`)  // "Hello, World!"
```

### Register Script Libraries

```go
p := scriptling.New()

// Register a library written in Scriptling
err := p.RegisterScriptLibrary("utils", `
def add(a, b):
    return a + b

def multiply(x, y):
    return x * y

PI = 3.14159

def circle_area(radius):
    return PI * radius * radius
`)
if err != nil {
    fmt.Println("Error:", err)
    return
}

// Use the registered library
p.Eval(`
import utils

result1 = utils.add(5, 3)
result2 = utils.multiply(4, 7)
area = utils.circle_area(5)

print("5 + 3 =", result1)
print("4 * 7 =", result2)
print("Area of circle with radius 5:", area)
`)
```

### Advanced Script Libraries

Script libraries can import other libraries and define complex functionality:

```go
p := scriptling.New()

// Register a data processing library
p.RegisterScriptLibrary("data_processor", `
import json
import math

def process_user_data(json_str):
    # Parse JSON data
    data = json.parse(json_str)

    # Calculate statistics
    if "scores" in data:
        scores = data["scores"]
        total = sum(scores)
        avg = total / len(scores)
        std_dev = math.sqrt(sum([(x - avg) ** 2 for x in scores]) / len(scores))

        return {
            "count": len(scores),
            "total": total,
            "average": avg,
            "std_dev": std_dev
        }
    else:
        return {"error": "No scores found"}

def validate_email(email):
    # Simple email validation
    return "@" in email and "." in email
`)

p.Eval(`
import data_processor

# Test the library
user_data = '{"name": "Alice", "scores": [85, 92, 78, 96, 88]}'
stats = data_processor.process_user_data(user_data)

print("User statistics:")
print("Count:", stats["count"])
print("Average:", stats["average"])
print("Std Dev:", stats["std_dev"])

# Test email validation
print("Valid email:", data_processor.validate_email("alice@example.com"))
print("Invalid email:", data_processor.validate_email("notanemail"))
`)
```

### Script Library Features

- **Multiple Functions**: Define as many functions as needed
- **Constants**: Define constants and variables
- **Nested Imports**: Script libraries can import other libraries
- **Complex Logic**: Full Scriptling syntax support
- **Error Handling**: Use try/except in library functions
- **Recursion**: Recursive functions work normally

## Performance Benefits

### Without Libraries
```go
p := scriptling.New()
p.Eval("x = 5 + 3")
```

### With Libraries
```go
p := scriptling.New()
p.Eval(`
    import requests, json
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
- Scriptling: `json.loads(string)` or `json["loads"](string)`

## Adding Libraries to Scriptling

To add a new standard library:

1. Create `stdlib/mylib.go`:
```go
package stdlib

import "github.com/paularlott/scriptling/object"

var MyLibrary = object.NewLibrary(map[string]*object.Builtin{
    "func1": { Fn: ... },
    "func2": { Fn: ... },
}, nil, "My library description")
```

2. Register in `scriptling.go`:
```go
var availableLibraries = map[string]*object.Library{
    "json":  stdlib.JSONLibrary,
    "math":  stdlib.MathLibrary,
    "mylib": stdlib.MyLibrary,  // Add here
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
