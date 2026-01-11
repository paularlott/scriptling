# Extending Scriptling - Libraries Guide

This guide covers how to create custom libraries for Scriptling in Go, including both the native API and the Builder API.

## Overview

Scriptling libraries group related functions and constants. You can create libraries using two approaches:

| Approach | When to Use |
|----------|-------------|
| **Native API** | Performance-critical code, complex state management |
| **Builder API** | Rapid development, typed functions, cleaner organization |

See [EXTENDING_WITH_GO.md](EXTENDING_WITH_GO.md) for a detailed comparison.

## Native API

### Basic Library Structure

```go
package main

import (
    "context"
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
)

func main() {
    p := scriptling.New()

    // Create a library
    myLib := object.NewLibrary(map[string]*object.Builtin{
        "add": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                if len(args) != 2 {
                    return &object.Error{Message: "add requires 2 arguments"}
                }
                a, ok1 := args[0].(*object.Integer)
                b, ok2 := args[1].(*object.Integer)
                if !ok1 || !ok2 {
                    return &object.Error{Message: "arguments must be integers"}
                }
                return &object.Integer{Value: a.Value + b.Value}
            },
            HelpText: "add(a, b) - Adds two numbers",
        },
        "multiply": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                if len(args) != 2 {
                    return &object.Error{Message: "multiply requires 2 arguments"}
                }
                a, ok1 := args[0].(*object.Integer)
                b, ok2 := args[1].(*object.Integer)
                if !ok1 || !ok2 {
                    return &object.Error{Message: "arguments must be integers"}
                }
                return &object.Integer{Value: a.Value * b.Value}
            },
            HelpText: "multiply(a, b) - Multiplies two numbers",
        },
    })

    // Register the library
    p.RegisterLibrary("mylib", myLib)

    // Use in script
    p.Eval(`
import mylib
result = mylib.add(5, 3)
print(result)  # 8
`)
}
```

### Library with Constants

Libraries can include constants that are accessible as library members:

```go
myLib := object.NewLibrary(
    map[string]*object.Builtin{
        "add": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                // Implementation
                return &object.Integer{Value: 0}
            },
            HelpText: "add(a, b) - Adds two numbers",
        },
    },
    map[string]object.Object{
        // Constants and classes go here
        "VERSION": &object.String{Value: "1.0.0"},
        "MAX_VALUE": &object.Integer{Value: 1000},
        "DEBUG": &object.Boolean{Value: false},
    },
    "My custom math library",  // Library description for help()
)

p.RegisterLibrary("mylib", myLib)

// Use in script
p.Eval(`
import mylib
print(mylib.VERSION)      # 1.0.0
print(mylib.MAX_VALUE)    # 1000
print(mylib.add(1, 2))    # 3
`)
```

### Library with State

Libraries can maintain state using Go closures:

```go
// Logger library that maintains state
type Logger struct {
    level    string
    messages []string
}

func NewLogger() *Logger {
    return &Logger{
        level:    "INFO",
        messages: make([]string, 0),
    }
}

func (l *Logger) CreateLibrary() map[string]*object.Builtin {
    return map[string]*object.Builtin{
        "set_level": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                if len(args) != 1 {
                    return &object.Error{Message: "set_level requires 1 argument"}
                }
                level, err := args[0].AsString()
                if err != nil {
                    return &object.Error{Message: "level must be string"}
                }
                l.level = level
                return &object.String{Value: "Level set to " + l.level}
            },
            HelpText: "set_level(level) - Set the logging level",
        },
        "log": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                if len(args) != 1 {
                    return &object.Error{Message: "log requires 1 argument"}
                }
                msg, err := args[0].AsString()
                if err != nil {
                    return &object.Error{Message: "message must be string"}
                }

                // Get environment for output
                env := evaluator.GetEnvFromContext(ctx)
                writer := env.GetWriter()

                logMsg := fmt.Sprintf("[%s] %s", l.level, msg)
                l.messages = append(l.messages, logMsg)
                fmt.Fprintln(writer, logMsg)

                return &object.String{Value: "logged"}
            },
            HelpText: "log(message) - Log a message",
        },
        "get_messages": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                elements := make([]object.Object, len(l.messages))
                for i, msg := range l.messages {
                    elements[i] = &object.String{Value: msg}
                }
                return &object.List{Elements: elements}
            },
            HelpText: "get_messages() - Get all logged messages",
        },
    }
}

// Usage
func main() {
    p := scriptling.New()
    logger := NewLogger()
    p.RegisterLibrary("logger", object.NewLibrary(logger.CreateLibrary(), nil, "Logger library"))

    p.Eval(`
import logger
logger.set_level("DEBUG")
logger.log("Application started")
logger.log("Processing data")
`)
}
```

### Sub-Libraries

Organize related functionality into sub-libraries:

```go
// Create URL parsing sub-library
parseLib := object.NewLibrary(
    map[string]*object.Builtin{
        "quote": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                s, _ := args[0].AsString()
                return &object.String{Value: url.QueryEscape(s)}
            },
            HelpText: "quote(s) - URL encode a string",
        },
        "unquote": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                s, _ := args[0].AsString()
                val, _ := url.QueryUnescape(s)
                return &object.String{Value: val}
            },
            HelpText: "unquote(s) - URL decode a string",
        },
    },
    nil,
    "URL parsing utilities",
)

// Create main URL library and add sub-library
urlLib := object.NewLibrary(
    map[string]*object.Builtin{
        "join": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                base, _ := args[0].AsString()
                path, _ := args[1].AsString()
                return &object.String{Value: strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/")}
            },
            HelpText: "join(base, path) - Join URL path segments",
        },
    },
    map[string]object.Object{
        "parse": parseLib,  // Sub-library as a constant
    },
    "URL utilities",
)

p.RegisterLibrary("url", urlLib)

// Use in script
p.Eval(`
import url
print(url.join("https://example.com", "/api/users"))  # https://example.com/api/users
print(url.parse.quote("hello world"))                   # hello+world
`)
```

## Builder API (Fluent Library)

The Builder API provides a cleaner, type-safe way to create libraries.

### Creating a Library

```go
import "github.com/paularlott/scriptling/object"

// Create builder
builder := object.NewLibraryBuilder("mymath", "Mathematical operations library")

// Register typed functions
builder.Function("add", func(a, b int) int {
    return a + b
})

builder.Function("multiply", func(a, b float64) float64 {
    return a * b
})

// Register constants
builder.Constant("PI", 3.14159)
builder.Constant("MAX_VALUE", 1000)

// Build the library
myLib := builder.Build()

// Register with Scriptling
p.RegisterLibrary("mymath", myLib)
```

### Supported Types

The Builder API automatically converts between Go types and Scriptling objects:

| Go Type | Scriptling Type | Notes |
|---------|-----------------|-------|
| `string` | `STRING` | Direct conversion |
| `int`, `int32`, `int64` | `INTEGER` | Accepts both Integer and Float |
| `float32`, `float64` | `FLOAT` | Accepts both Integer and Float |
| `bool` | `BOOLEAN` | Direct conversion |
| `[]any` | `LIST` | Converts to/from Scriptling lists |
| `map[string]any` | `DICT` | Converts to/from Scriptling dicts |

### Function Signatures

The Builder API supports flexible function signatures:

- `func(args...) result` - Positional arguments only
- `func(ctx context.Context, args...) result` - Context + positional arguments
- `func(kwargs object.Kwargs, args...) result` - Kwargs + positional arguments
- `func(ctx context.Context, kwargs object.Kwargs, args...) result` - All parameters
- `func(kwargs object.Kwargs) result` - Kwargs only
- `func(ctx context.Context, kwargs object.Kwargs) result` - Context + kwargs only

### Examples

**Simple functions:**

```go
builder.Function("sqrt", func(x float64) float64 {
    return math.Sqrt(x)
})

builder.Function("power", func(base, exp float64) float64 {
    return math.Pow(base, exp)
})
```

**With context:**

```go
builder.Function("timeout_op", func(ctx context.Context, timeout int) error {
    select {
    case <-time.After(time.Duration(timeout) * time.Second):
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
})
```

**With kwargs:**

```go
builder.Function("connect", func(kwargs object.Kwargs) (string, error) {
    host, err := kwargs.GetString("host", "localhost")
    if err != nil {
        return "", err
    }
    port, err := kwargs.GetInt("port", 8080)
    if err != nil {
        return "", err
    }
    return fmt.Sprintf("%s:%d", host, port), nil
})
```

**With error handling:**

```go
builder.Function("safe_divide", func(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
})
```

**With complex types:**

```go
builder.Function("sum_list", func(items []any) float64 {
    sum := 0.0
    for _, item := range items {
        if v, ok := item.(float64); ok {
            sum += v
        }
    }
    return sum
})

builder.Function("process_config", func(config map[string]any) string {
    if host, ok := config["host"].(string); ok {
        return "Connected to " + host
    }
    return "No host"
})
```

### Adding Help Text

```go
builder.FunctionWithHelp("sqrt", func(x float64) float64 {
    return math.Sqrt(x)
}, "sqrt(x) - Return the square root of x")

builder.FunctionWithHelp("divide", func(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
}, "divide(a, b) - Divide two numbers (returns error if b is zero)")
```

### Constants

```go
builder.Constant("VERSION", "1.0.0")
builder.Constant("MAX_CONNECTIONS", 100)
builder.Constant("DEBUG_MODE", true)
builder.Constant("DEFAULT_TIMEOUT", 30.5)
```

### Sub-Libraries

```go
// Create URL parsing sub-library
parseBuilder := object.NewLibraryBuilder("parse", "URL parsing utilities")
parseBuilder.Function("quote", func(s string) string {
    return url.QueryEscape(s)
})
parseBuilder.Function("unquote", func(s string) string {
    val, _ := url.QueryUnescape(s)
    return val
})
parseLib := parseBuilder.Build()

// Create main URL library and add sub-library
urlBuilder := object.NewLibraryBuilder("url", "URL utilities")
urlBuilder.Function("join", func(base, path string) string {
    return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(path, "/")
})
urlBuilder.SubLibrary("parse", parseLib)
urlLib := urlBuilder.Build()
```

### Complete Example

```go
package main

import (
    "fmt"
    "math"

    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
)

func main() {
    p := scriptling.New()

    // Create a math library using the Builder API
    mathBuilder := object.NewLibraryBuilder("mymath", "Advanced math operations")

    // Basic operations
    mathBuilder.Function("add", func(a, b int) int {
        return a + b
    })

    mathBuilder.FunctionWithHelp("multiply", func(a, b float64) float64 {
        return a * b
    }, "multiply(a, b) - Multiply two numbers")

    // Advanced operations with error handling
    mathBuilder.FunctionWithHelp("divide", func(a, b float64) (float64, error) {
        if b == 0 {
            return 0, fmt.Errorf("division by zero")
        }
        return a / b, nil
    }, "divide(a, b) - Divide two numbers (returns error if b is zero)")

    mathBuilder.Function("sqrt", func(x float64) float64 {
        return math.Sqrt(x)
    })

    mathBuilder.Function("power", func(base, exp float64) float64 {
        return math.Pow(base, exp)
    })

    // Constants
    mathBuilder.Constant("PI", math.Pi)
    mathBuilder.Constant("E", math.E)
    mathBuilder.Constant("GoldenRatio", 1.618)

    // Build and register the library
    myMath := mathBuilder.Build()
    p.RegisterLibrary("mymath", myMath)

    // Use the library
    p.Eval(`
import mymath

# Basic operations
print("2 + 3 =", mymath.add(2, 3))
print("4 * 5 =", mymath.multiply(4.0, 5.0))
print("10 / 2 =", mymath.divide(10.0, 2.0))
print("sqrt(16) =", mymath.sqrt(16.0))
print("2^8 =", mymath.power(2.0, 8.0))

# Constants
print("PI =", mymath.PI)
print("E =", mymath.E)
`)
}
```

### Builder Methods Reference

| Method | Description |
|--------|-------------|
| `Function(name, fn)` | Register a typed Go function |
| `FunctionWithHelp(name, fn, help)` | Register a function with help text |
| `Constant(name, value)` | Register a constant value |
| `RawFunction(name, fn)` | Register a low-level builtin function |
| `SubLibrary(name, lib)` | Add a sub-library |
| `FunctionFromVariadic(name, fn)` | Register a variadic function |
| `Alias(alias, original)` | Create an alias for an existing function |
| `Build()` | Create and return the Library |
| `Clear()` | Remove all registered functions and constants |
| `Merge(other)` | Merge another builder's functions and constants |

## Choosing Between Native and Builder API

| Factor | Native API | Builder API |
|--------|------------|-------------|
| **Performance** | Faster | Slight overhead |
| **Type Safety** | Manual checking | Automatic conversion |
| **State Management** | Full control with closures | Clean organization |
| **Help Text** | Manual `HelpText` field | Chainable `FunctionWithHelp()` |
| **Best For** | Complex state, performance-critical | Rapid development, typed functions |

## Best Practices

### 1. Group Related Functions

```go
// Good: Organized by functionality
mathLib := object.NewLibrary(map[string]*object.Builtin{
    "add": {...},
    "subtract": {...},
    "multiply": {...},
})

stringLib := object.NewLibrary(map[string]*object.Builtin{
    "upper": {...},
    "lower": {...},
    "trim": {...},
})
```

### 2. Provide Descriptive Help Text

```go
"add": {
    Fn: func(...) { ... },
    HelpText: `add(a, b) - Add two numbers

  Parameters:
    a - First number
    b - Second number

  Returns:
    The sum of a and b

  Examples:
    add(2, 3)  # Returns 5`,
}
```

### 3. Use Constants for Configuration

```go
configLib := object.NewLibrary(
    map[string]*object.Builtin{
        "get_config": {...},
    },
    map[string]object.Object{
        "API_VERSION": &object.String{Value: "v1"},
        "TIMEOUT": &object.Integer{Value: 30},
        "DEBUG": &object.Boolean{Value: false},
    },
    "Configuration library",
)
```

### 4. Add Library Description

```go
myLib := object.NewLibrary(
    functions,
    constants,
    "My custom data processing library",  // Description shown by help()
)
```

## Testing Libraries

```go
func TestLibrary(t *testing.T) {
    p := scriptling.New()

    // Create and register library
    lib := object.NewLibrary(map[string]*object.Builtin{
        "add": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                a, _ := args[0].(*object.Integer)
                b, _ := args[1].(*object.Integer)
                return &object.Integer{Value: a.Value + b.Value}
            },
        },
    }, nil, "Test library")
    p.RegisterLibrary("testlib", lib)

    // Test the library
    result, err := p.Eval(`
import testlib
result = testlib.add(10, 20)
`)
    if err != nil {
        t.Fatalf("Eval error: %v", err)
    }

    if value, objErr := p.GetVarAsInt("result"); objErr == nil {
        if value != 30 {
            t.Errorf("Expected 30, got %d", value)
        }
    }
}
```

## See Also

- **[EXTENDING_WITH_GO.md](EXTENDING_WITH_GO.md)** - Overview and common concepts
- **[EXTENDING_FUNCTIONS.md](EXTENDING_FUNCTIONS.md)** - Creating individual functions
- **[EXTENDING_CLASSES.md](EXTENDING_CLASSES.md)** - Defining classes
- **[EXTENDING_WITH_SCRIPTS.md](EXTENDING_WITH_SCRIPTS.md)** - Creating extensions in Scriptling
- **[HELP_SYSTEM.md](HELP_SYSTEM.md)** - Adding documentation
