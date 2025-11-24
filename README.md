# Scriptling - Python-like Scripting Language for Go

A minimal, sandboxed interpreter for LLM agents to execute code and interact with REST APIs. Python-inspired syntax designed for embedding in Go applications.

## Features

- **Python-like syntax** with indentation-based blocks
- **Core types**: integers, floats, strings, booleans, lists, dictionaries
- **Control flow**: if/elif/else, while, for loops, break, continue, pass
- **Advanced features**: range(), slice notation, dict methods (keys/values/items)
- **Functions** with recursion support
- **Optional libraries**: JSON and HTTP (load on demand)
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
numbers = [1, 2, 3]            # List
person = {"name": "Bob"}        # Dictionary

# Arithmetic (division always returns float)
result = 5 / 2                  # 2.5 (true division like Python 3)
remainder = 5 % 2               # 1 (modulo)

# Augmented assignment
x += 10                         # x = x + 10
pi *= 2                         # pi = pi * 2
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

### Libraries
```python
# Import libraries dynamically
import("json")
import("http")

# Use JSON (dot notation)
data = json.parse('{"name":"Alice"}')
json_str = json.stringify({"key": "value"})

# Use HTTP (dot notation)
response = http.get("https://api.example.com/data", 10)
response = http.post(url, body, 10)

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
- **List operations**: `len()`, `append()`
- **Import**: `import("library_name")`

## Libraries

### JSON Library
```python
import("json")

# Parse JSON
data = json.parse('{"name":"Alice","age":30}')

# Stringify
json_str = json.stringify({"key": "value"})
```

### HTTP Library
```python
import("http")

# All methods return {"status": int, "body": string, "headers": dict}
# All methods support optional options dictionary
# Default timeout: 5 seconds

# Simple requests (5 second timeout)
response = http.get(url)
response = http.post(url, body)
response = http.put(url, body)
response = http.delete(url)
response = http.patch(url, body)

# With options (timeout and/or headers)
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123"}
}
response = http.get(url, options)
response = http.post(url, body, options)
response = http.put(url, body, options)
response = http.delete(url, options)
response = http.patch(url, body, options)

# Check status
if response["status"] == 200:
    print(response["body"])
```

## Examples

See `examples/` directory:
- `main.go` - Complete Go integration example
- `basic.py` - Basic language features
- `functions.py` - Functions and recursion
- `collections.py` - Lists, dicts, for loops
- `rest_api.py` - REST API calls
- `rest_api_lib.py` - REST API with library syntax

Run example:
```bash
cd examples
go run main.go
```

## Documentation

- **README.md** (this file) - Quick start and overview
- **LANGUAGE_GUIDE.md** - Complete language reference
- **GO_INTEGRATION.md** - Go integration and embedding guide
- **LIBRARIES.md** - Library system and custom libraries
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

## License

MIT

## Contributing

Issues and pull requests welcome at [github.com/paularlott/scriptling](https://github.com/paularlott/scriptling)
