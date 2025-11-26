# Scriptling - Python-like Scripting Language for Go

A minimal, sandboxed interpreter for LLM agents to execute code and interact with REST APIs. Python-inspired syntax designed for embedding in Go applications.

## Features

- **Python-like syntax** with indentation-based blocks
- **Core types**: integers, floats, strings, booleans, lists, dictionaries
- **Control flow**: if/elif/else, while, for loops, break, continue, pass
- **Advanced features**: range(), slice notation, dict methods (keys/values/items), multiple assignment, keyword arguments
- **List comprehensions**: `[x * x for x in range(10) if x > 5]`
- **Method call syntax**: `text.upper()`, `json.loads()`, `math.sqrt(16)`
- **Lambda functions**: `square = lambda x: x * x`
- **Default parameters**: `def greet(name, greeting="Hello"):`
- **Tuple literals**: `point = (1, 2)` with unpacking support
- **Functions** with recursion support
- **Error handling**: try/except/finally, raise statement
- **Optional libraries**: json, http, re, math, time (load on demand)
- **Go integration**: Register functions, exchange variables
- **Fast**: Lightweight interpreter, only loads what you need

## Installation

```bash
go get github.com/paularlott/scriptling
```

## Quick Start

```go
package main

import (
    "fmt"
    "github.com/paularlott/scriptling"
)

func main() {
    // Create interpreter
    p := scriptling.New()

    // Optional: Register Requests library if needed
    p.RegisterLibrary("http", stdlib.HTTPLibrary())

    // Execute Scriptling code
    _, err := p.Eval(`
# Import libraries as needed
import json
import requests

# Make API call
response = http.get("https://api.example.com/data", 10)
if response["status"] == 200:
    data = json.parse(response["body"])
    print(data["name"])
`)

    if err != nil {
        fmt.Println("Error:", err)
    }
}
```

## Language Overview

### Variables and Types
```python
x = 42                          # Integer
pi = 3.14                       # Float
name = "Alice"                  # String
flag = True                     # Boolean
result = None                   # None (null value)
numbers = [1, 2, 3]            # List
person = {"name": "Bob"}        # Dictionary

# Arithmetic (division always returns float)
result = 5 / 2                  # 2.5 (true division like Python 3)
remainder = 5 % 2               # 1 (modulo)

# Augmented assignment
x += 10                         # x = x + 10
pi *= 2                         # pi = pi * 2
```

### Membership Operators
```python
# in operator
if 5 in [1, 2, 3, 4, 5]:
    print("Found in list")

if "name" in {"name": "Alice", "age": 30}:
    print("Key exists")

if "hello" in "hello world":
    print("Substring found")

# not in operator
if 10 not in [1, 2, 3]:
    print("Not in list")
```

### Control Flow
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

# Loop control
for i in [1, 2, 3, 4, 5]:
    if i == 3:
        continue  # Skip 3
    if i == 5:
        break     # Stop at 5
    print(i)
```

### Functions
```python
def add(a, b):
    return a + b

result = add(5, 3)
```

### Error Handling
```python
try:
    response = http.get("https://api.example.com/data", {"timeout": 5})
    if response["status"] != 200:
        raise "HTTP error: " + str(response["status"])
    data = json.parse(response["body"])
except:
    print("Request failed")
    data = None
finally:
    print("Cleanup complete")
```

### Multiple Assignment
```python
# Unpack lists
a, b = [1, 2]
x, y, z = [10, 20, 30]

# Swap variables
x, y = [y, x]

# From function or expression
first, second = [1 + 1, 2 * 2]
```

### List Comprehensions
```python
# Basic comprehension
squares = [x * x for x in range(5)]

# With condition
evens = [x for x in range(10) if x % 2 == 0]

# String processing
chars = [c.upper() for c in "hello"]
```

### Method Call Syntax
```python
# String methods
text = "hello world"
result = text.upper().replace("WORLD", "SCRIPTLING")

# Library methods
import json
data = json.parse('{"name": "Alice"}')
name = data["name"].upper()
```

### Lambda Functions & Default Parameters
```python
# Lambda functions
square = lambda x: x * x
add = lambda a, b: a + b

# Default parameters
def greet(name, greeting="Hello"):
    return greeting + " " + name

print(greet("Alice"))        # "Hello Alice"
print(greet("Bob", "Hi"))    # "Hi Bob"
```

### Tuple Literals
```python
# Tuple creation and unpacking
point = (1, 2)
x, y = point

# Mixed types
user = ("Alice", 30, True)
name, age, active = user
```

### Libraries
```python
# Import libraries dynamically. The import statement loads the library
# and makes its functions available as a global object.
import json    # Creates global 'json' object
import requests    # Creates global 'requests' object
import re   # Creates global 'regex' object

# Use JSON (dot notation)
data = json.parse('{"name":"Alice"}')
json_str = json.stringify({"key": "value"})

# Use HTTP (dot notation)
options = {"timeout": 10}
response = http.get("https://api.example.com/data", options)
response = http.post(url, body, options)

# Use Regex (dot notation)
matches = re.findall("[0-9]+", "abc123def456")

# Bracket notation also works
data = json["parse"]('...')
response = http["get"](...)
```

## Go API

### Create Interpreter
```go
// Create interpreter (libraries loaded via import)
p := scriptling.New()

// Optional: Register Requests library if needed
p.RegisterLibrary("http", stdlib.HTTPLibrary())
```

### Execute Code
```go
result, err := p.Eval("x = 5 + 3")
```

### Exchange Variables
```go
// Set from Go
p.SetVar("api_key", "secret123")
p.SetVar("timeout", 30)

// Get from Scriptling
value, ok := p.GetVar("result")
```

### Register Go Functions
```go
p.RegisterFunc("custom", func(ctx context.Context, args ...object.Object) object.Object {
    // Your Go code here
    return &object.String{Value: "result"}
})

p.Eval(`output = custom()`)
```

### Register Custom Libraries
```go
myLib := map[string]*object.Builtin{
    "hello": {
        Fn: func(ctx context.Context, args ...object.Object) object.Object {
            return &object.String{Value: "Hello!"}
        },
    },
}

p.RegisterLibrary("mylib", myLib)
p.Eval(`mylib.hello()`)  // or mylib["hello"]()
```

## Built-in Functions

Always available without loading libraries:

- **I/O**: `print(value)`
- **Type conversions**: `str()`, `int()`, `float()`
- **String operations**: `len()`, `upper()`, `lower()`, `split()`, `join()`, `replace()`
- **List operations**: `len()`, `append()`, `range()`
- **Dictionary operations**: `keys()`, `values()`, `items()`

**See [docs/LANGUAGE_GUIDE.md](docs/LANGUAGE_GUIDE.md) for complete language reference.**

## Libraries

Scriptling includes optional libraries for common tasks:

- **json** - Parse and stringify JSON
- **re** - Regular expressions (match, find, findall, replace, split)
- **math** - Mathematical functions (sqrt, pow, abs, floor, ceil, round, min, max, pi, e)
- **time** - Time operations (time, perf_counter, sleep, strftime, strptime)
- **base64** - Base64 encoding/decoding
- **hashlib** - Hashing functions (md5, sha1, sha256)
- **random** - Random number generation
- **lib** - URL parsing and manipulation (Python urllib.parse compatible)

**Optional Libraries** (require manual registration):
- **http** - Make HTTP requests (GET, POST, PUT, DELETE, PATCH)
  ```go
  import "github.com/paularlott/scriptling/extlibs"
  p.RegisterLibrary("http", extlibs.HTTPLibrary())
  ```

**Quick Example:**
```python
import json
import math
import lib

# Parse JSON data
data = json.loads('{"radius": 5}')  # Python-compatible API

# Calculate something
radius = data["radius"]
area = math.pi * math.pow(radius, 2)

# URL manipulation
encoded = lib.quote("hello world")  # Python urllib.parse.quote()
print("Area: " + str(area) + ", Encoded: " + encoded)
```

**HTTP Example** (requires `p.RegisterLibrary("http", extlibs.HTTPLibrary())`):
```python
import json
import requests

# Make API call
response = http.get("https://api.example.com/data", {"timeout": 10})
if response["status"] == 200:
    data = json.parse(response["body"])
    print(data["name"])
```

**See [docs/LIBRARIES.md](docs/LIBRARIES.md) for complete library documentation.**

## Examples

See `examples/` directory:
- **scripts/** - Script examples and Go integration
  - `main.go` - Complete Go integration example
  - `test_*.py` - Comprehensive test scripts
  - `benchmark.py` - Performance benchmark script
- **mcp/** - MCP server for LLM testing
  - `main.go` - MCP server implementation
  - `README.md` - Usage instructions

Run script examples:
```bash
cd examples/scripts
go run main.go test_basics.py
go run main.go test_error_comprehensive.py
./run_all_tests.sh
```

Run MCP server for LLM testing:
```bash
cd examples/mcp
go mod tidy
go run main.go
```

**Note**: HTTP tests will fail unless Requests library is registered in main.go (this is intentional - HTTP is an optional extra).

Run benchmark:
```bash
go test -v -run=TestBenchmarkScript
```

## MCP Server for LLM Testing

The `examples/mcp/` directory contains an MCP (Model Context Protocol) server that allows LLMs to:
- Execute Scriptling code and see results
- Learn about differences between Scriptling and Python
- Discover available libraries and their usage

This enables LLMs to test and understand Scriptling interactively.

## Documentation

- **README.md** (this file) - Quick start and overview
- **docs/LANGUAGE_GUIDE.md** - Complete language reference
- **docs/GO_INTEGRATION.md** - Go integration and embedding guide
- **docs/EXTENDING_SCRIPTLING.md** - Creating custom functions and libraries
- **docs/LIBRARIES.md** - Library system and custom libraries
- **docs/QUICK_REFERENCE.md** - Quick reference guide

## File Extension

Scriptling scripts use `.py` extension for syntax highlighting in editors. While not Python, the syntax is similar enough for highlighters to work well.

## Testing

```bash
# Run all tests
go test ./...

# Run specific package
go test ./evaluator -v
```

40+ tests, 29/31 passing (HTTP tests require manual registration).

## Use Cases

- **Configuration scripts** - Dynamic configuration with logic
- **REST API automation** - Make HTTP calls, process JSON
- **Embedded scripting** - Add scripting to your Go application
- **Data processing** - Transform and manipulate data
- **Automation tasks** - Scriptable workflows

## Performance

- **Lightweight**: Core interpreter has minimal overhead
- **On-demand loading**: Only load JSON/HTTP when needed
- **Fast execution**: Optimized for embedded use
- **Global cache**: Compiled scripts cached globally with LRU eviction
- **Regex caching**: Compiled regex patterns cached with LRU eviction
- **Thread safety**: Each Scriptling environment is single-threaded. Create separate environments for concurrent use.

## Thread Safety

**Important**: Scriptling environments are **not thread-safe**. Each environment instance must be used by only one Go thread at a time. For concurrent execution:

- Create separate `Scriptling` instances for each thread
- Each instance maintains its own environment and variables
- Global caches (scripts, regex patterns) are thread-safe and shared across instances
- Do not share a single environment across multiple goroutines

```go
// Correct: Separate instances for concurrent use
go func() {
    p1 := scriptling.New()
    p1.Eval("x = 1")
}()

go func() {
    p2 := scriptling.New()  // Separate instance
    p2.Eval("y = 2")
}()
```

## License

MIT

## Contributing

Issues and pull requests welcome at [github.com/paularlott/scriptling](https://github.com/paularlott/scriptling)
