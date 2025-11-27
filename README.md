<div align="center">
<img src="docs/images/mascot_small.png" alt="Scriptling" />

![GitHub License](https://img.shields.io/github/license/paularlott/scriptling)

</div>

# Scriptling - Python-like Scripting Language for Go

A minimal, sandboxed interpreter for LLM agents to execute code and interact with REST APIs. Python-inspired syntax designed for embedding in Go applications.

## Features

- **Python-like syntax** with indentation-based blocks
- **Core types**: integers, floats, strings, booleans, lists, dictionaries
- **Control flow**: if/elif/else, while, for loops, break, continue
- **Advanced features**: functions, lambda, list comprehensions, error handling
- **Libraries**: json, regex, math, time, http (load on demand)
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

    // Execute Scriptling code
    result, err := p.Eval(`
# Variables and types
x = 42
name = "Alice"
numbers = [1, 2, 3]

# Functions
def greet(n):
    return "Hello " + n

# Output
print(greet(name))
print("Sum:", x + len(numbers))
`)

    if err != nil {
        fmt.Println("Error:", err)
    }
}
```

## Go API

### Basic Usage
```go
p := scriptling.New()

// Execute code
result, err := p.Eval("x = 5 + 3")

// Exchange variables
p.SetVar("name", "Alice")
value, ok := p.GetVarAsString("name")

// Register Go functions
p.RegisterFunc("custom", func(ctx context.Context, args ...object.Object) object.Object {
    return &object.String{Value: "result"}
})

// Register Scriptling functions
p.RegisterScriptFunc("my_func", `
def my_func(x):
    return x * 2
my_func
`)

// Register Scriptling libraries
p.RegisterScriptLibrary("mylib", `
def add(a, b):
    return a + b
PI = 3.14159
`)
```

### Libraries
```go
// Register optional libraries
import "github.com/paularlott/scriptling/extlibs"
p.RegisterLibrary("http", extlibs.HTTPLibrary())

// Use in scripts
p.Eval(`
import json
import http

response = http.get("https://api.example.com/data")
data = json.parse(response["body"])
`)
```

## Examples

See `examples/` directory:
- **scripts/** - Script examples and Go integration
- **mcp/** - MCP server for LLM testing

Run examples:
```bash
cd examples/scripts
go run main.go test_basics.py
```

## Documentation

- **[Language Guide](docs/LANGUAGE_GUIDE.md)** - Complete syntax reference
- **[Libraries](docs/LIBRARIES.md)** - Available libraries and APIs
- **[Go Integration](docs/GO_INTEGRATION.md)** - Embedding and extending
- **[Quick Reference](docs/QUICK_REFERENCE.md)** - Cheat sheet

## Testing

```bash
# Run all tests
go test ./...

# Run benchmarks
go test -bench=. -run=Benchmark
```

## License

MIT

## Contributing

Issues and pull requests welcome at [github.com/paularlott/scriptling](https://github.com/paularlott/scriptling)
