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
- **Object-oriented**: Classes with single inheritance, methods, and constructors
- **Advanced features**: functions, lambda, list comprehensions, error handling
- **Libraries**: including json, regex, math, time, requests, subprocess (load on demand)
- **Go integration**: Register functions, exchange variables
- **Fast**: Lightweight interpreter, only loads what you need
- **Sandboxed**: No filesystem or network access unless enabled via libraries
- **MCP support**: Model Context Protocol for LLM integration
- **CLI tool**: Run scripts, start HTTP server with MCP support
- **Extensible**: Create custom functions, libraries, and classes in Go or Scriptling
- **Linter**: Built-in linter for syntax checks
- **OpenAPI support**: Generate Scriptling client libraries from OpenAPI specs with o2s tool

## Differences from Python

While Scriptling is inspired by Python, it has some key differences:

- **Single Inheritance Only**: Classes support single inheritance (e.g., `class Dog(Animal):`), but not multiple inheritance.
- **No `del` Statement**: Use slice assignment instead — e.g. `lst = lst[:n]` rather than `del lst[n:]`.
- **No Nested Classes**: Classes cannot be defined within other classes.
- **Simplified Scope**: `nonlocal` and `global` keywords work slightly differently.
- **Go Integration**: Designed primarily for embedding in Go, with direct type mapping.
- **Sandboxed**: No direct access to filesystem or network unless explicitly enabled via libraries.

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
    "github.com/paularlott/scriptling/stdlib"
)

func main() {
    // Create interpreter
    p := scriptling.New()

    // Register all standard libraries
    stdlib.RegisterAll(p)

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

## CLI Tool

Scriptling includes a command-line interface for running scripts directly:

```bash
# Install Task (build tool)
brew install go-task/tap/go-task

# Build CLI for current platform
task build

# Run scripts
./bin/scriptling script.py
echo 'print("Hello")' | ./bin/scriptling
./bin/scriptling --interactive

# Build for all platforms
task build-all
```

### HTTP & MCP Server

The CLI can also run as an HTTP server with MCP (Model Context Protocol) support for LLM integration:

```bash
# Start HTTP server on port 8000
./bin/scriptling --server :8000 script.py

# With MCP tools for LLM integration
./bin/scriptling --server :8000 --mcp-tools ./tools script.py

# With TLS and authentication
./bin/scriptling --server :8443 --tls-generate --bearer-token secret script.py
```

See [scriptling-cli/README.md](scriptling-cli/README.md) for details.

## Go API

### Basic Usage

```go
import (
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/stdlib"
)

p := scriptling.New()

// Register libraries as needed
stdlib.RegisterAll(p)  // Register all standard libraries
// Or register individual libraries:
// p.RegisterLibrary(stdlib.JSONLibraryName, stdlib.JSONLibrary)
// p.RegisterLibrary(stdlib.MathLibraryName, stdlib.MathLibrary)

// Execute code
result, err := p.Eval("x = 5 + 3")

// Exchange variables
p.SetVar("name", "Alice")
value, objErr := p.GetVarAsString("name")

// Register Go functions
p.RegisterFunc("custom", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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

// Set on-demand library callback for dynamic loading
p.SetOnDemandLibraryCallback(func(p *Scriptling, libName string) bool {
    if libName == "disklib" {
        // Load library from disk and register it
        script, err := loadLibraryFromDisk(libName)
        if err != nil {
            return false
        }
        return p.RegisterScriptLibrary(libName, script) == nil
    }
    return false
})
```

### Libraries

```go
import (
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/stdlib"
    "github.com/paularlott/scriptling/extlibs"
)

// Create interpreter
p := scriptling.New()

// Register all standard libraries
stdlib.RegisterAll(p)

// Or register individual standard libraries
p.RegisterLibrary(stdlib.JSONLibrary)
p.RegisterLibrary(stdlib.MathLibrary)

// Register additional custom libraries
p.RegisterLibrary(extlibs.RequestsLibrary)

// Register os and pathlib with security restrictions
extlibs.RegisterOSLibrary(p, []string{"/tmp", "/home/user/data"})
extlibs.RegisterPathlibLibrary(p, []string{"/tmp", "/home/user/data"})

// Import libraries programmatically (no need for import statements in scripts)
p.Import("json")

// Use in scripts
p.Eval(`
import requests
import os

response = requests.get("https://api.example.com/data")
data = json.parse(response["body"])  # json already imported via p.Import()
files = os.listdir("/tmp")
`)
```

## Examples

See `examples/` directory:

- **scripts/** - Script examples and Go integration
- **openai/scriptlingcoder/** - AI coding assistant with custom tools (inspired by [nanocode](https://github.com/1rgs/nanocode))
- **mcp-tools/** - MCP tools for use with CLI MCP server

Run examples:

```bash
cd examples/scripts
go run main.go test_basics.py

# AI coding assistant (⚠️ executes AI-generated code)
cd examples/openai/scriptlingcoder
../../../bin/scriptling scriptlingcoder.py
```

## Tools

- **[o2s](tools/o2s/README.md)** - OpenAPI to Scriptling converter. Generate HTTP client libraries from OpenAPI v3 specs

## Documentation

Full documentation is available at **[scriptling.dev](https://scriptling.dev)**

- **[Quick Start](https://scriptling.dev/docs/quick-start/)** - Get up and running quickly
- **[Language Guide](https://scriptling.dev/docs/language/)** - Complete syntax reference
- **[Go Integration](https://scriptling.dev/docs/go-integration/)** - Embedding Scriptling in Go applications
- **[Libraries](https://scriptling.dev/docs/libraries/)** - Standard and extended library reference
- **[CLI Reference](https://scriptling.dev/docs/cli/)** - Command-line tool documentation

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

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.
