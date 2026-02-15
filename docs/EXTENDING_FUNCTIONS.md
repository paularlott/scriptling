# Extending Scriptling - Functions Guide

This guide covers how to create custom functions for Scriptling in Go, including both the native API and the Builder API.

## Overview

Scriptling functions can be created using two approaches:

| Approach | When to Use |
|----------|-------------|
| **Native API** | Performance-critical code, complex logic, full control |
| **Builder API** | Rapid development, simpler functions, cleaner syntax |

See [EXTENDING_WITH_GO.md](EXTENDING_WITH_GO.md) for a detailed comparison.

## Function Signature

All Scriptling functions use a unified signature:

```go
func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
```

- `ctx`: Context for cancellation and environment access
- `kwargs`: Keyword arguments wrapper with helper methods
- `args`: Positional arguments as Scriptling objects
- Returns: A Scriptling object result

## Native API

### Simple Function (Positional Arguments Only)

For functions that only use positional arguments, you can ignore the `kwargs` parameter:

```go
package main

import (
    "context"
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
    "github.com/paularlott/scriptling/stdlib"
)

func main() {
    p := scriptling.New()
    stdlib.RegisterAll(p)

    // Register a simple function
    p.RegisterFunc("double", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
        if len(args) != 1 {
            return &object.Error{Message: "double requires 1 argument"}
        }

        if intObj, ok := args[0].(*object.Integer); ok {
            return &object.Integer{Value: intObj.Value * 2}
        }

        return &object.Error{Message: "argument must be integer"}
    })

    // Use from Scriptling
    p.Eval(`
result = double(21)
print(result)  # 42
`)
}
```

### Function with Keyword Arguments

Functions can accept keyword arguments using the `kwargs` wrapper:

```go
// Function with keyword arguments only
p.RegisterFunc("make_duration", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Reject positional arguments
    if len(args) > 0 {
        return &object.Error{Message: "make_duration takes no positional arguments"}
    }

    // Use kwargs helper methods with defaults
    hours, err := kwargs.GetFloat("hours", 0.0)
    if err != nil {
        return &object.Error{Message: err.Error()}
    }

    minutes, err := kwargs.GetFloat("minutes", 0.0)
    if err != nil {
        return &object.Error{Message: err.Error()}
    }

    seconds, err := kwargs.GetFloat("seconds", 0.0)
    if err != nil {
        return &object.Error{Message: err.Error()}
    }

    totalSeconds := hours*3600 + minutes*60 + seconds
    return &object.Float{Value: totalSeconds}
})

// Use from Scriptling
p.Eval(`
duration = make_duration(hours=2, minutes=30)
print(duration)  # 9000.0
`)
```

### Function with Mixed Positional and Keyword Arguments

```go
p.RegisterFunc("format_greeting", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    if len(args) != 1 {
        return &object.Error{Message: "format_greeting requires name argument"}
    }

    name, err := args[0].AsString()
    if err != nil {
        return &object.Error{Message: "name must be string"}
    }

    // Use kwargs helper methods with defaults
    prefix, err := kwargs.GetString("prefix", "Hello")
    if err != nil {
        return &object.Error{Message: err.Error()}
    }

    suffix, err := kwargs.GetString("suffix", "!")
    if err != nil {
        return &object.Error{Message: err.Error()}
    }

    return &object.String{Value: prefix + ", " + name + suffix}
})

// Use from Scriptling
p.Eval(`
print(format_greeting("World"))                    # Hello, World!
print(format_greeting("World", prefix="Hi"))       # Hi, World!
print(format_greeting("World", suffix="..."))      # Hello, World...
print(format_greeting("World", prefix="Hey", suffix="?"))  # Hey, World?
`)
```

### Kwargs Helper Methods

The `object.Kwargs` type provides convenient helper methods:

| Method | Description |
|--------|-------------|
| `GetString(name, default) (string, error)` | Extract string, return default if missing |
| `GetInt(name, default) (int64, error)` | Extract int (accepts Integer/Float) |
| `GetFloat(name, default) (float64, error)` | Extract float (accepts Integer/Float) |
| `GetBool(name, default) (bool, error)` | Extract bool |
| `GetList(name, default) ([]Object, error)` | Extract list elements |
| `Has(name) bool` | Check if key exists |
| `Keys() []string` | Get all keys |
| `Len() int` | Get number of kwargs |
| `Get(name) Object` | Get raw Object value |

#### Must* Variants (No Error Handling)

For simple cases where you want to use defaults on any error:

| Method | Description |
|--------|-------------|
| `MustGetString(name, default) string` | Extract string, ignore errors |
| `MustGetInt(name, default) int64` | Extract int, ignore errors |
| `MustGetFloat(name, default) float64` | Extract float, ignore errors |
| `MustGetBool(name, default) bool` | Extract bool, ignore errors |
| `MustGetList(name, default) []Object` | Extract list, ignore errors |

### Type-Safe Accessor Methods

All Scriptling objects implement type-safe accessor methods that simplify type checking:

```go
// Instead of manual type assertions
p.RegisterFunc("add_tax", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    if len(args) != 2 {
        return &object.Error{Message: "add_tax requires 2 arguments"}
    }

    // Manual type assertion (tedious!)
    priceObj, ok := args[0].(*object.Float)
    if !ok {
        intObj, ok := args[0].(*object.Integer)
        if !ok {
            return &object.Error{Message: "first argument must be number"}
        }
        priceObj = &object.Float{Value: float64(intObj.Value)}
    }

    rateObj, ok := args[1].(*object.Float)
    if !ok {
        intObj, ok := args[1].(*object.Integer)
        if !ok {
            return &object.Error{Message: "second argument must be number"}
        }
        rateObj = &object.Float{Value: float64(intObj.Value)}
    }

    result := priceObj.Value * (1 + rateObj.Value)
    return &object.Float{Value: result}
})

// Using type-safe accessors (clean!)
p.RegisterFunc("add_tax", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    if len(args) != 2 {
        return &object.Error{Message: "add_tax requires 2 arguments"}
    }

    // AsFloat() automatically handles both Integer and Float
    price, err := args[0].AsFloat()
    if err != nil {
        return &object.Error{Message: "price: " + err.Error()}
    }

    rate, err := args[1].AsFloat()
    if err != nil {
        return &object.Error{Message: "rate: " + err.Error()}
    }

    result := price * (1 + rate)
    return &object.Float{Value: result}
})
```

#### Available Accessor Methods

| Method | Description |
|--------|-------------|
| `AsString() (string, error)` | Extract string value |
| `AsInt() (int64, error)` | Extract integer (floats truncate) |
| `AsFloat() (float64, error)` | Extract float (ints convert automatically) |
| `AsBool() (bool, error)` | Extract boolean |
| `AsList() ([]Object, error)` | Extract list/tuple elements (returns a copy) |
| `AsDict() (map[string]Object, error)` | Extract dict as map (keys are human-readable strings) |

### Function with Output Capture

Functions can write to the output capture system:

```go
import (
    "context"
    "fmt"
    "github.com/paularlott/scriptling/evaluator"
)

p.RegisterFunc("debug_print", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Get environment from context
    env := evaluator.GetEnvFromContext(ctx)
    writer := env.GetWriter()

    // Print debug information
    fmt.Fprintf(writer, "[DEBUG] Function called with %d arguments\n", len(args))
    for i, arg := range args {
        fmt.Fprintf(writer, "[DEBUG] Arg %d: %s\n", i, arg.Inspect())
    }

    return &object.String{Value: "logged"}
})
```

### Adding Help Text

```go
p.RegisterFunc("calculate", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Implementation
    return &object.Integer{Value: 42}
}, `calculate(x, y) - Perform calculation

  Parameters:
    x - First number
    y - Second number

  Returns:
    The calculated result

  Examples:
    calculate(10, 5)  # Returns 15
`)

// Users can then access help:
// help("calculate")  # Shows the documentation
```

If you omit the help text, basic help will be auto-generated:

```go
p.RegisterFunc("my_func", func(...) object.Object {
    // Auto-generates: "my_func(...) - User-defined function"
    return object.NULL
})
```

### Advanced Example: File Operations with Output Capture

```go
import (
    "os"
    "github.com/paularlott/scriptling/evaluator"
)

p.RegisterFunc("read_file", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    if len(args) != 1 {
        return &object.Error{Message: "read_file requires 1 argument"}
    }

    path, err := args[0].AsString()
    if err != nil {
        return &object.Error{Message: "path must be string"}
    }

    // Get environment for output capture
    env := evaluator.GetEnvFromContext(ctx)
    writer := env.GetWriter()

    content, err := os.ReadFile(path)
    if err != nil {
        fmt.Fprintf(writer, "Error reading file: %s\n", err.Error())
        return &object.String{Value: ""}
    }

    fmt.Fprintf(writer, "Successfully read %d bytes from %s\n", len(content), path)
    return &object.String{Value: string(content)}
}
```

## Builder API (Fluent Function)

The Builder API provides a cleaner, type-safe way to create functions with automatic type conversion.

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
| `nil` | `None` | Null value |

### Function Signatures

The Builder API supports flexible function signatures:

- `func(args...) result` - Positional arguments only
- `func(ctx context.Context, args...) result` - Context + positional arguments
- `func(kwargs object.Kwargs, args...) result` - Kwargs + positional arguments
- `func(ctx context.Context, kwargs object.Kwargs, args...) result` - All parameters
- `func(kwargs object.Kwargs) result` - Kwargs only
- `func(ctx context.Context, kwargs object.Kwargs) result` - Context + kwargs only

**Parameter Order Rules (ALWAYS in this order):**
1. Context (optional) - comes first if present
2. Kwargs (optional) - comes after context (or first if no context)
3. Positional arguments - ALWAYS LAST

### Examples

**Simple positional arguments:**

```go
fb := object.NewFunctionBuilder()
fb.Function(func(a, b int) int {
    return a + b
})
p.RegisterFunc("add", fb.Build())

// Usage: add(3, 4) → 7
```

**With context:**

```go
fb := object.NewFunctionBuilder()
fb.Function(func(ctx context.Context, timeout int) error {
    select {
    case <-time.After(time.Duration(timeout) * time.Second):
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
})
p.RegisterFunc("wait", fb.Build())
```

**With kwargs:**

```go
fb := object.NewFunctionBuilder()
fb.Function(func(kwargs object.Kwargs) (string, error) {
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
p.RegisterFunc("connect", fb.Build())

// Usage: connect(host="example.com", port=443) → "example.com:443"
```

**Mixed positional and kwargs:**

```go
fb := object.NewFunctionBuilder()
fb.Function(func(kwargs object.Kwargs, name string, count int) string {
    prefix, _ := kwargs.GetString("prefix", ">")
    return fmt.Sprintf("%s %s: %d", prefix, name, count)
})
p.RegisterFunc("log", fb.Build())

// Usage: log("task", 5, prefix=">>>") → ">>> task: 5"
```

**With error handling:**

```go
fb := object.NewFunctionBuilder()
fb.Function(func(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
})
p.RegisterFunc("divide", fb.Build())
```

### Adding Help Text

```go
fb := object.NewFunctionBuilder()
fb.FunctionWithHelp(func(x float64) float64 {
    return math.Sqrt(x)
}, "sqrt(x) - Return the square root of x")
p.RegisterFunc("sqrt", fb.Build())
```

### Builder Methods Reference

| Method | Description |
|--------|-------------|
| `Function(fn)` | Register a typed Go function |
| `FunctionWithHelp(fn, help)` | Register with help text |
| `Build()` | Return the BuiltinFunction |

## Choosing Between Native and Builder API

| Factor | Native API | Builder API |
|--------|------------|-------------|
| **Performance** | Faster (no reflection overhead) | Slight overhead |
| **Code Clarity** | More verbose | Cleaner |
| **Type Safety** | Manual checking | Automatic |
| **Flexibility** | Full control | Convention-based |
| **Best For** | Performance-critical, complex logic | Rapid development, simple functions |

## Testing Custom Functions

```go
func TestCustomFunction(t *testing.T) {
    p := scriptling.New()
    p.EnableOutputCapture()

    // Register test function
    p.RegisterFunc("test_func", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
        return &object.String{Value: "success"}
    })

    // Test the function
    result, err := p.Eval(`result = test_func()`)
    if err != nil {
        t.Fatalf("Eval error: %v", err)
    }

    // Check return value
    if value, objErr := p.GetVarAsString("result"); objErr == nil {
        if value != "success" {
            t.Errorf("Expected 'success', got '%s'", value)
        }
    }
}
```

## See Also

- **[EXTENDING_WITH_GO.md](EXTENDING_WITH_GO.md)** - Overview and common concepts
- **[EXTENDING_LIBRARIES.md](EXTENDING_LIBRARIES.md)** - Creating libraries
- **[EXTENDING_CLASSES.md](EXTENDING_CLASSES.md)** - Defining classes
- **[EXTENDING_WITH_SCRIPTS.md](EXTENDING_WITH_SCRIPTS.md)** - Creating extensions in Scriptling
- **[HELP_SYSTEM.md](HELP_SYSTEM.md)** - Adding documentation
