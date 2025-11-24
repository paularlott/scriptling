# Scriptling - Python-like Scripting Language for Go

A minimal, sandboxed interpreter for LLM agents to execute code and interact with REST APIs. Python-inspired syntax designed for embedding in Go applications.

## Features

- **Python-like syntax** with indentation-based blocks
- **Core types**: integers, floats, strings, booleans, lists, dictionaries
- **Control flow**: if/elif/else, while, for loops, break, continue, pass
- **Advanced features**: range(), slice notation, dict methods (keys/values/items), multiple assignment
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
    // Create interpreter with libraries
    p := scriptling.New("json", "http")

    // Execute Scriptling code
    _, err := p.Eval(`
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

### Libraries
```python
# Import libraries dynamically. The import statement loads the library
# and makes its functions available as a global object.
import json    # Creates global 'json' object
import http    # Creates global 'http' object
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
// No libraries (lightweight)
p := scriptling.New()

// With specific libraries
p := scriptling.New("json")
p := scriptling.New("json", "http")
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
p.RegisterFunc("custom", func(args ...object.Object) object.Object {
    // Your Go code here
    return &object.String{Value: "result"}
})

p.Eval(`output = custom()`)
```

### Register Custom Libraries
```go
myLib := map[string]*object.Builtin{
    "hello": {
        Fn: func(args ...object.Object) object.Object {
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
- **http** - Make HTTP requests (GET, POST, PUT, DELETE, PATCH)
- **re** - Regular expressions (match, find, findall, replace, split)
- **math** - Mathematical functions (sqrt, pow, abs, floor, ceil, round, min, max, pi, e)
- **time** - Time operations (time, perf_counter, sleep, strftime, strptime)

**Quick Example:**
```python
import json
import http
import math

# Make API call
response = http.get("https://api.example.com/data", {"timeout": 10})
if response["status"] == 200:
    data = json.parse(response["body"])
    
    # Calculate something
    radius = data["radius"]
    area = math.pi() * math.pow(radius, 2)
    print("Area: " + str(area))
```

**See [docs/LIBRARIES.md](docs/LIBRARIES.md) for complete library documentation.**

## Examples

See `examples/` directory:
- `main.go` - Complete Go integration example
- `basic.py` - Basic language features
- `functions.py` - Functions and recursion
- `collections.py` - Lists, dicts, for loops
- `error_handling_test.py` - Error handling basics
- `error_handling_comprehensive.py` - Comprehensive error handling examples
- `error_handling_http.py` - Error handling with HTTP and JSON
- `rest_api.py` - REST API calls
- `rest_api_lib.py` - REST API with library syntax
- `benchmark.py` - Performance benchmark script

Run example:
```bash
cd examples
go run main.go basic.py
go run main.go error_handling_comprehensive.py
```

Run all tests:
```bash
cd examples
./run_all_tests.sh
```

Run benchmark:
```bash
go test -v -run=TestBenchmarkScript
```

## Documentation

- **README.md** (this file) - Quick start and overview
- **docs/LANGUAGE_GUIDE.md** - Complete language reference
- **docs/GO_INTEGRATION.md** - Go integration and embedding guide
- **docs/LIBRARIES.md** - Library system and custom libraries
- **docs/QUICK_REFERENCE.md** - Quick reference guide
- **BUILD_PLAN.md** - Architecture and build progress

## File Extension

Scriptling scripts use `.py` extension for syntax highlighting in editors. While not Python, the syntax is similar enough for highlighters to work well.

## Testing

```bash
# Run all tests
go test ./...

# Run specific package
go test ./evaluator -v
```

42 tests, 100% passing.

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
- **Thread-safe**: Safe for concurrent use across multiple instances

## License

MIT

## Contributing

Issues and pull requests welcome at [github.com/paularlott/scriptling](https://github.com/paularlott/scriptling)
