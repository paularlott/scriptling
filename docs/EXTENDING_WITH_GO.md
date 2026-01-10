# Extending Scriptling - Libraries and Functions Guide

This guide covers how to create custom libraries and functions for Scriptling in Go, including advanced features like output capture and keyword arguments.

## Function Signature

All Scriptling functions in Go use a unified signature that supports both positional and keyword arguments:

```go
func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object
```

- `ctx`: Context containing environment and other runtime information
- `kwargs`: Map of keyword arguments (may be `nil` if no kwargs passed)
- `args`: Variable number of positional Scriptling objects passed from the script
- Returns: A Scriptling object result

## Basic Function Creation

### Simple Function (Positional Arguments Only)

For functions that only use positional arguments, you can simply ignore the `kwargs` parameter:

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
    stdlib.RegisterAll(p)  // Register standard libraries if needed

    // Register a simple function - kwargs ignored
    p.RegisterFunc("double", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
        if len(args) != 1 {
            return &object.String{Value: "Error: double requires 1 argument"}
        }

        if intObj, ok := args[0].(*object.Integer); ok {
            return &object.Integer{Value: intObj.Value * 2}
        }

        return &object.String{Value: "Error: argument must be integer"}
    })

    // Use from Scriptling
    p.Eval(`
result = double(21)
print(result)  # 42
`)
}
```

### Function with Keyword Arguments

Functions can accept keyword arguments using the `kwargs` map:

```go
// timedelta-style function with keyword arguments only
p.RegisterFunc("make_duration", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    // Reject positional arguments
    if len(args) > 0 {
        return &object.String{Value: "Error: make_duration takes no positional arguments"}
    }

    var hours, minutes, seconds float64

    // Process keyword arguments
    for key, val := range kwargs {
        var num float64
        switch v := val.(type) {
        case *object.Integer:
            num = float64(v.Value)
        case *object.Float:
            num = v.Value
        default:
            return &object.String{Value: "Error: argument must be numeric"}
        }

        switch key {
        case "hours":
            hours = num
        case "minutes":
            minutes = num
        case "seconds":
            seconds = num
        default:
            return &object.String{Value: "Error: unexpected keyword argument: " + key}
        }
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

Functions can accept both positional and keyword arguments:

```go
// Function with required positional arg and optional kwargs
p.RegisterFunc("format_greeting", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) != 1 {
        return &object.String{Value: "Error: format_greeting requires name argument"}
    }

    name, ok := args[0].(*object.String)
    if !ok {
        return &object.String{Value: "Error: name must be string"}
    }

    // Default values
    prefix := "Hello"
    suffix := "!"

    // Override with kwargs if provided
    if kwargs != nil {
        if val, exists := kwargs["prefix"]; exists {
            if s, ok := val.(*object.String); ok {
                prefix = s.Value
            }
        }
        if val, exists := kwargs["suffix"]; exists {
            if s, ok := val.(*object.String); ok {
                suffix = s.Value
            }
        }
    }

    return &object.String{Value: prefix + ", " + name.Value + suffix}
})

// Use from Scriptling
p.Eval(`
print(format_greeting("World"))                    # Hello, World!
print(format_greeting("World", prefix="Hi"))       # Hi, World!
print(format_greeting("World", suffix="..."))      # Hello, World...
print(format_greeting("World", prefix="Hey", suffix="?"))  # Hey, World?
`)
```

### Function with Output Capture

```go
import (
    "context"
    "fmt"
    "github.com/paularlott/scriptling/evaluator"
)

p.RegisterFunc("debug_print", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    // Get environment from context to access output capture
    env := evaluator.GetEnvFromContext(ctx)
    writer := env.GetWriter()

    // Print debug information
    fmt.Fprintf(writer, "[DEBUG] Function called with %d arguments\n", len(args))
    for i, arg := range args {
        fmt.Fprintf(writer, "[DEBUG] Arg %d: %s\n", i, arg.Inspect())
    }

    return &object.String{Value: "debug complete"}
})
```

## Type-Safe Accessor Methods

All Scriptling objects implement type-safe accessor methods that simplify type checking and value extraction. These methods return `(value, ok)` tuples similar to Go's map access pattern.

### Available Accessor Methods

Every `object.Object` implements these methods:

```go
AsString() (string, bool)            // Extract string value
AsInt() (int64, bool)                // Extract integer value
AsFloat() (float64, bool)            // Extract float value (auto-converts integers)
AsBool() (bool, bool)                // Extract boolean value
AsList() ([]Object, bool)            // Extract list/tuple elements
AsDict() (map[string]Object, bool)   // Extract dict as map (keys are strings)
```

### Benefits Over Type Assertions

**Using type assertions:**

```go
p.RegisterFunc("add_tax", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) != 2 {
        return errors.NewArgumentError(len(args), 2)
    }

    // Manual type assertion and value extraction - tedious!
    priceObj, ok := args[0].(*object.Float)
    if !ok {
        // Need to handle Integer separately
        intObj, ok := args[0].(*object.Integer)
        if !ok {
            return errors.NewTypeError("NUMBER", args[0].Type().String())
        }
        priceObj = &object.Float{Value: float64(intObj.Value)}
    }

    rateObj, ok := args[1].(*object.Float)
    if !ok {
        intObj, ok := args[1].(*object.Integer)
        if !ok {
            return errors.NewTypeError("NUMBER", args[1].Type().String())
        }
        rateObj = &object.Float{Value: float64(intObj.Value)}
    }

    result := priceObj.Value * (1 + rateObj.Value)
    return &object.Float{Value: result}
})
```

**Using type-safe accessors:**

```go
p.RegisterFunc("add_tax", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) != 2 {
        return errors.NewArgumentError(len(args), 2)
    }

    // Automatic type coercion - Integer.AsFloat() returns the float value!
    price, ok := args[0].AsFloat()
    if !ok {
        return errors.NewTypeError("NUMBER", args[0].Type().String())
    }

    rate, ok := args[1].AsFloat()
    if !ok {
        return errors.NewTypeError("NUMBER", args[1].Type().String())
    }

    result := price * (1 + rate)
    return &object.Float{Value: result}
})
```

### Key Advantages

1. **Automatic Type Coercion**: `Integer.AsFloat()` returns `(float64(value), true)` automatically
2. **Cleaner Code**: No need to access `.Value` field after type assertion
3. **Consistent Pattern**: Same `(value, ok)` pattern throughout
4. **Dictionary Simplification**: `AsDict()` returns `map[string]Object` instead of `map[string]DictPair`

### Working with Strings

```go
p.RegisterFunc("greet", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) != 1 {
        return errors.NewArgumentError(len(args), 1)
    }

    // Clean string extraction - one line!
    name, ok := args[0].AsString()
    if !ok {
        return errors.NewTypeError("STRING", args[0].Type().String())
    }

    return &object.String{Value: "Hello, " + name + "!"}
})
```

### Working with Lists

```go
p.RegisterFunc("sum_list", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) != 1 {
        return errors.NewArgumentError(len(args), 1)
    }

    // Extract list elements directly - no .Elements needed
    elements, ok := args[0].AsList()
    if !ok {
        return errors.NewTypeError("LIST", args[0].Type().String())
    }

    var sum float64
    for _, elem := range elements {
        // AsFloat() works on both Integer and Float
        val, ok := elem.AsFloat()
        if !ok {
            return errors.NewError("all elements must be numeric")
        }
        sum += val
    }

    return &object.Float{Value: sum}
})
```

### Working with Dictionaries

```go
p.RegisterFunc("process_config", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) != 1 {
        return errors.NewArgumentError(len(args), 1)
    }

    // Get dict as simple map[string]Object - no .Pairs!
    config, ok := args[0].AsDict()
    if !ok {
        return errors.NewTypeError("DICT", args[0].Type().String())
    }

    // Access values directly - clean and simple
    if hostVal, exists := config["host"]; exists {
        if host, ok := hostVal.AsString(); ok {
            fmt.Printf("Host: %s\n", host)
        }
    }

    if portVal, exists := config["port"]; exists {
        if port, ok := portVal.AsInt(); ok {
            fmt.Printf("Port: %d\n", port)
        }
    }

    return &object.String{Value: "processed"}
})
```

### Type Coercion Reference

| Object Type | AsString() | AsInt() | AsFloat()     | AsBool() | AsList() | AsDict() |
| ----------- | ---------- | ------- | ------------- | -------- | -------- | -------- |
| String      | ✓ value    | ✗       | ✗             | ✓ len>0  | ✗        | ✗        |
| Integer     | ✗          | ✓ value | **✓ float64** | ✓ val≠0  | ✗        | ✗        |
| Float       | ✗          | ✗       | ✓ value       | ✓ val≠0  | ✗        | ✗        |
| Boolean     | ✗          | ✗       | ✗             | ✓ value  | ✗        | ✗        |
| List        | ✗          | ✗       | ✗             | ✓ len>0  | ✓ elems  | ✗        |
| Tuple       | ✗          | ✗       | ✗             | ✓ len>0  | ✓ elems  | ✗        |
| Dict        | ✗          | ✗       | ✗             | ✓ len>0  | ✗        | ✓ map    |
| Null        | ✗          | ✗       | ✗             | ✓ false  | ✗        | ✗        |

**Note**: Bold entries indicate automatic type coercion (e.g., Integer → float64).

**Recommendation**: Always use type-safe accessors (`AsString()`, `AsInt()`, etc.) instead of direct type assertions when implementing custom functions.

## Argument Extraction Helpers

The `scriptling` package provides helper functions in `conversion.go` that simplify argument extraction with automatic error generation. These helpers combine type checking and value extraction in a single call.

### Available Helper Functions

```go
import "github.com/paularlott/scriptling"

// Required positional argument extractors
GetString(args, index, name) (string, object.Object)
GetInt(args, index, name) (int64, object.Object)
GetFloat(args, index, name) (float64, object.Object)
GetBool(args, index, name) (bool, object.Object)
GetList(args, index, name) ([]object.Object, object.Object)
GetDict(args, index, name) (map[string]object.Object, object.Object)

// Optional positional argument extractors
GetStringOptional(args, index, name, defaultValue) (string, bool, object.Object)
GetIntOptional(args, index, name, defaultValue) (int64, bool, object.Object)

// Keyword argument extractors (with default values)
GetStringFromKwargs(kwargs, name, defaultValue) (string, object.Object)
GetIntFromKwargs(kwargs, name, defaultValue) (int64, object.Object)
GetFloatFromKwargs(kwargs, name, defaultValue) (float64, object.Object)
GetBoolFromKwargs(kwargs, name, defaultValue) (bool, object.Object)
```

### Keyword Argument Helpers

The kwargs helpers make working with optional named parameters much cleaner:

**Without helpers (verbose):**
```go
p.RegisterFunc("connect", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    var timeout int64 = 30
    if t, ok := kwargs["timeout"]; ok {
        if i, ok := t.AsInt(); ok {
            timeout = i
        } else {
            return errors.NewError("timeout: must be a number")
        }
    }

    var debug bool = false
    if d, ok := kwargs["debug"]; ok {
        if b, ok := d.AsBool(); ok {
            debug = b
        } else {
            return errors.NewError("debug: must be a boolean")
        }
    }

    var protocol string = "tcp"
    if p, ok := kwargs["protocol"]; ok {
        if s, ok := p.AsString(); ok {
            protocol = s
        } else {
            return errors.NewError("protocol: must be a string")
        }
    }

    // ... rest of function
})
```

**With helpers (clean):**
```go
p.RegisterFunc("connect", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    timeout, err := scriptling.GetIntFromKwargs(kwargs, "timeout", 30)
    if err != nil {
        return err
    }
    debug, err := scriptling.GetBoolFromKwargs(kwargs, "debug", false)
    if err != nil {
        return err
    }
    protocol, err := scriptling.GetStringFromKwargs(kwargs, "protocol", "tcp")
    if err != nil {
        return err
    }

    // ... rest of function
})
```

### Kwargs Helper Behavior

All kwargs helpers:
1. **Return the value (nil error)** from kwargs if the key exists and type matches
2. **Return the default value (nil error)** if key is missing
3. **Return an error** if key exists but type doesn't match (helps catch typos like "timout" vs "timeout")
4. **Accept type coercion** - `GetIntFromKwargs` accepts both Integer and Float, `GetFloatFromKwargs` accepts both Integer and Float

### Complete Example with Positional and Keyword Arguments

**Without helpers (verbose):**
```go
p.RegisterFunc("connect", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) < 2 {
        return errors.NewError("connect() requires at least 2 arguments")
    }

    var host string
    if s, ok := args[0].AsString(); ok {
        host = s
    } else {
        return errors.NewError("host: must be a string")
    }

    var port int64
    if i, ok := args[1].AsInt(); ok {
        port = i
    } else {
        return errors.NewError("port: must be an integer")
    }

    // ... rest of function
})
```

**With helpers (concise):**
```go
p.RegisterFunc("connect", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    host, err := scriptling.GetString(args, 0, "host")
    if err != nil {
        return err
    }

    port, err := scriptling.GetInt(args, 1, "port")
    if err != nil {
        return err
    }

    // Optional timeout parameter
    timeout, hasTimeout, err := scriptling.GetIntOptional(args, 2, "timeout", 30)
    if err != nil {
        return err
    }
    if !hasTimeout {
        // Use default timeout
    }

    // ... rest of function
})
```

### Helper Function Behavior

All helper functions:
1. **Check bounds**: Return an error if `index >= len(args)`
2. **Type check**: Return an error if the argument is the wrong type
3. **Return value**: Return the extracted value on success
4. **Auto-generate errors**: Error messages include the argument name for clarity

### Error Message Format

```go
// Missing argument
GetString(args, 5, "filename")
// → Error: "filename: missing argument"

// Wrong type
GetInt(args, 0, "count")
// → Error: "count: must be an integer"

// Success
GetString(args, 0, "path")
// → ("some/path", nil)
```

### Benefits

1. **Less boilerplate**: Combine bounds check, type check, and extraction
2. **Consistent errors**: Automatic, descriptive error messages
3. **Easier refactoring**: Argument name is specified once
4. **Optional support**: Built-in handling for optional parameters

### Integration with Type-Safe Accessors

The helpers use the type-safe accessor methods internally, so you get all the benefits:
- `GetFloat()` accepts both Integer and Float (auto-converts)
- `GetList()` works with both List and Tuple
- Clean, idiomatic Go error handling

## Fluent Library API

The Fluent Library API provides a clean, type-safe way to create Scriptling libraries using regular Go functions with typed parameters. The API automatically handles conversion between Go types and Scriptling objects, eliminating boilerplate code.

### Overview

**Without Fluent API (verbose):**

```go
myLib := object.NewLibrary(map[string]*object.Builtin{
    "connect": {
        Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
            if len(args) < 2 {
                return errors.NewArgumentError(len(args), 2)
            }
            host, ok := args[0].AsString()
            if !ok {
                return errors.NewTypeError("STRING", args[0].Type().String())
            }
            port, ok := args[1].AsInt()
            if !ok {
                return errors.NewTypeError("INTEGER", args[1].Type().String())
            }
            // ... rest of function
            return &object.Null{}
        },
    },
    "disconnect": {
        Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
            // ... implementation
            return &object.Null{}
        },
    },
})
```

**With Fluent API (clean):**

```go
lib := object.NewLibraryBuilder("mylib", "My custom library")
lib.Function("connect", func(host string, port int) error {
    // Connect to host:port
    return nil
})
lib.Function("disconnect", func() error {
    // Disconnect
    return nil
})
myLib := lib.Build()
```

### Creating a Library

Use `object.NewLibraryBuilder()` to create a new builder, then register functions and constants:

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

The Fluent API automatically converts between Go types and Scriptling objects:

| Go Type              | Scriptling Type | Notes                              |
| -------------------- | --------------- | ----------------------------------- |
| `string`             | `STRING`        | Direct conversion                  |
| `int`, `int32`, `int64` | `INTEGER`    | Accepts both Integer and Float     |
| `float32`, `float64` | `FLOAT`         | Accepts both Integer and Float     |
| `bool`               | `BOOLEAN`       | Direct conversion                  |
| `[]any`              | `LIST`          | Converts to/from Scriptling lists  |
| `map[string]any`     | `DICT`          | Converts to/from Scriptling dicts  |
| `nil`                | `None`          | Null value                         |

### Function Signatures

Functions can have various signatures:

**Single return value:**
```go
builder.Function("sqrt", func(x float64) float64 {
    return math.Sqrt(x)
})
```

**Return value + error:**
```go
builder.Function("divide", func(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("division by zero")
    }
    return a / b, nil
})
```

**No return value:**
```go
builder.Function("print", func(msg string) {
    fmt.Println(msg)
})
```

**Multiple parameters:**
```go
builder.Function("format", func(name string, age int, score float64) string {
    return fmt.Sprintf("%s (age %d) scored %.2f", name, age, score)
})
```

**Complex types:**
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

### Keyword Arguments (Kwargs)

The Fluent API supports keyword arguments through the `scriptling.Kwargs` type. When used as the last parameter in a function signature, the builder automatically passes the keyword arguments map to it.

**Kwargs-only function:**
```go
import "github.com/paularlott/scriptling"

builder.Function("connect", func(kwargs scriptling.Kwargs) (string, error) {
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

// Usage in Scriptling:
// connect(host="example.com", port=443)  # → "example.com:443"
// connect(port=9000)                      # → "localhost:9000"
// connect()                               # → "localhost:8080"
```

**Mixed positional and kwargs:**
```go
builder.Function("format", func(name string, count int, kwargs scriptling.Kwargs) (string, error) {
    prefix, err := kwargs.GetString("prefix", ">")
    if err != nil {
        return "", err
    }
    suffix, err := kwargs.GetString("suffix", "<")
    if err != nil {
        return "", err
    }
    return fmt.Sprintf("%s %s: %d times %s", prefix, name, count, suffix), nil
})

// Usage in Scriptling:
// format("task", 5)                          # → "> task: 5 times <"
// format("task", 3, prefix=">>>")            # → ">>> task: 3 times <"
// format("task", 10, suffix="<<<")           # → "> task: 10 times <<<"
// format("task", 7, prefix="[", suffix="]")  # → "[ task: 7 times ]"
```

#### Kwargs Helper Methods

The `scriptling.Kwargs` type provides helper methods for extracting values with defaults:

| Method | Description |
|--------|-------------|
| `GetString(name, default) (string, error)` | Extract string, return default if missing |
| `GetInt(name, default) (int64, error)` | Extract int (accepts Integer/Float) |
| `GetFloat(name, default) (float64, error)` | Extract float (accepts Integer/Float) |
| `GetBool(name, default) (bool, error)` | Extract bool |
| `Has(name) bool` | Check if key exists |
| `Keys() []string` | Get all keys |
| `Len() int` | Get number of kwargs |
| `Get(name) object.Object` | Get raw Object value |

**Must* variants (panic on error):**
```go
builder.Function("quick", func(kwargs scriptling.Kwargs) string {
    // Must helpers panic on error - use for simple cases
    timeout := kwargs.MustGetInt("timeout", 30)
    debug := kwargs.MustGetBool("debug", false)
    return fmt.Sprintf("timeout=%d debug=%t", timeout, debug)
})
```

#### Error Handling

Kwargs helpers return errors when:
- The kwarg is provided but has an incompatible type
- This helps catch typos like passing `"123"` instead of `123`

```go
builder.Function("connect", func(kwargs scriptling.Kwargs) (string, error) {
    port, err := kwargs.GetInt("port", 8080)
    if err != nil {
        // err.Error() = "port: must be a number"
        return "", err
    }
    // ...
})
```

### Adding Help Text

Provide documentation for your functions using `FunctionWithHelp`:

```go
builder.FunctionWithHelp("sqrt", func(x float64) float64 {
    return math.Sqrt(x)
}, "sqrt(x) - Return the square root of x")

builder.FunctionWithHelp("connect", func(host string, port int) error {
    // implementation
    return nil
}, "connect(host, port) - Connect to the specified host and port")
```

### Constants

Register constants that are available in the library:

```go
builder.Constant("VERSION", "1.0.0")
builder.Constant("MAX_CONNECTIONS", 100)
builder.Constant("DEBUG_MODE", true)
builder.Constant("DEFAULT_TIMEOUT", 30.5)
```

### Raw Functions (Advanced)

For advanced use cases where you need direct access to the low-level API, use `RawFunction`:

```go
builder.RawFunction("custom", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    // Direct access to context, kwargs, and raw objects
    env := evaluator.GetEnvFromContext(ctx)
    // ... implementation
    return &object.String{Value: "result"}
})
```

### Sub-Libraries

Organize related functions into sub-libraries:

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

### Variadic Functions

Register functions that accept variable arguments:

```go
builder.FunctionFromVariadic("print_all", func(args ...any) {
    for _, arg := range args {
        fmt.Println(arg)
    }
})

builder.FunctionFromVariadic("sum", func(numbers ...int) int {
    total := 0
    for _, n := range numbers {
        total += n
    }
    return total
})
```

### Function Aliases

Create aliases for existing functions:

```go
builder.Function("add", func(a, b int) int {
    return a + b
})
builder.Alias("sum", "add")  // "sum" is now an alias for "add"
```

### Builder Methods Reference

| Method                     | Description                                              |
| -------------------------- | -------------------------------------------------------- |
| `Function(name, fn)`       | Register a typed Go function                             |
| `FunctionWithHelp(...)`    | Register a function with help text                        |
| `Constant(name, value)`    | Register a constant value                                |
| `RawFunction(name, fn)`    | Register a low-level builtin function                     |
| `SubLibrary(name, lib)`    | Add a sub-library                                        |
| `FunctionFromVariadic(...)`| Register a variadic function                             |
| `Alias(alias, original)`   | Create an alias for an existing function                 |
| `Build()`                  | Create and return the Library                            |
| `Clear()`                  | Remove all registered functions and constants            |
| `Merge(other)`             | Merge another builder's functions and constants           |
| `FunctionCount()`          | Get number of registered functions                       |
| `ConstantCount()`          | Get number of registered constants                       |
| `HasFunction(name)`        | Check if a function is registered                        |
| `HasConstant(name)`        | Check if a constant is registered                        |
| `GetFunctionNames()`       | Get sorted list of function names                        |
| `GetConstantNames()`       | Get sorted list of constant names                        |

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

    // Create a math library using the Fluent API
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

    // Complex operations
    mathBuilder.Function("sum_list", func(numbers []any) float64 {
        sum := 0.0
        for _, num := range numbers {
            if v, ok := num.(float64); ok {
                sum += v
            } else if v, ok := num.(int64); ok {
                sum += float64(v)
            }
        }
        return sum
    })

    mathBuilder.Function("stats", func(data []any) map[string]any {
        if len(data) == 0 {
            return map[string]any{"count": 0, "mean": 0.0, "sum": 0.0}
        }

        sum := 0.0
        count := 0
        for _, v := range data {
            if num, ok := v.(float64); ok {
                sum += num
                count++
            }
        }

        return map[string]any{
            "count": count,
            "mean":  sum / float64(count),
            "sum":   sum,
        }
    })

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

# Complex operations
numbers = [1.0, 2.0, 3.0, 4.0, 5.0]
print("Sum of", numbers, "=", mymath.sum_list(numbers))

stats = mymath.stats(numbers)
print("Stats:", stats)
`)
}
```

### Error Handling

The Fluent API automatically handles errors returned from functions:

```go
// Function returning (result, error)
builder.Function("safe_divide", func(a, b float64) (float64, error) {
    if b == 0 {
        return 0, fmt.Errorf("cannot divide by zero")
    }
    return a / b, nil
})

// Usage in Scriptling:
// result = mymath.safe_divide(10, 0)  # Returns Error object
```

### Type Conversion

The API automatically handles type conversions:

```go
// Integer works for float64 parameter
builder.Function("halve", func(x float64) float64 {
    return x / 2
})
// mymath.halve(5) works - converts to 2.5

// Float works for int64 parameter
builder.Function("double", func(x int) int {
    return x * 2
})
// mymath.double(3.14) works - converts to 6 (truncates)
```

## Advanced Function Examples

### File Operations with Output Capture

```go
p.RegisterFunc("read_file", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) != 1 {
        return &object.String{Value: "Error: read_file requires 1 argument"}
    }

    pathObj, ok := args[0].(*object.String)
    if !ok {
        return &object.String{Value: "Error: path must be string"}
    }

    // Get environment for output capture
    env := evaluator.GetEnvFromContext(ctx)
    writer := env.GetWriter()

    content, err := os.ReadFile(pathObj.Value)
    if err != nil {
        fmt.Fprintf(writer, "Error reading file: %s\n", err.Error())
        return &object.String{Value: ""}
    }

    fmt.Fprintf(writer, "Successfully read %d bytes from %s\n", len(content), pathObj.Value)
    return &object.String{Value: string(content)}
})
````

### HTTP Client with Logging

```go
import (
    "net/http"
    "io"
    "time"
)

p.RegisterFunc("http_get", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) != 1 {
        return &object.String{Value: "Error: http_get requires 1 argument"}
    }

    urlObj, ok := args[0].(*object.String)
    if !ok {
        return &object.String{Value: "Error: URL must be string"}
    }

    // Get environment for logging
    env := evaluator.GetEnvFromContext(ctx)
    writer := env.GetWriter()

    fmt.Fprintf(writer, "Making HTTP GET request to: %s\n", urlObj.Value)

    client := &http.Client{Timeout: 10 * time.Second}
    resp, err := client.Get(urlObj.Value)
    if err != nil {
        fmt.Fprintf(writer, "HTTP request failed: %s\n", err.Error())
        return &object.String{Value: ""}
    }
    defer resp.Body.Close()

    body, err := io.ReadAll(resp.Body)
    if err != nil {
        fmt.Fprintf(writer, "Failed to read response body: %s\n", err.Error())
        return &object.String{Value: ""}
    }

    fmt.Fprintf(writer, "HTTP request completed: %d bytes received\n", len(body))
    return &object.String{Value: string(body)}
})
```

**Note**: You can provide documentation for your function by passing help text as an optional parameter to `RegisterFunc`, which will be displayed by the `help()` function:

```go
p.RegisterFunc("my_func", myFunc, "my_func() - Description...")
```

If you omit the help text or pass an empty string, basic help will be auto-generated:

```go
p.RegisterFunc("my_func", myFunc)  // Auto-generates: "my_func(...) - User-defined function"
```

## Creating Custom Libraries

### Basic Library Structure

```go
package mylib

import (
    "context"
    "github.com/paularlott/scriptling/object"
)

// CreateMathLibrary creates a custom math library
func CreateMathLibrary() map[string]*object.Builtin {
    return map[string]*object.Builtin{
        "add": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                if len(args) != 2 {
                    return &object.String{Value: "Error: add requires 2 arguments"}
                }

                var a, b float64

                // Handle integers and floats
                switch arg := args[0].(type) {
                case *object.Integer:
                    a = float64(arg.Value)
                case *object.Float:
                    a = arg.Value
                default:
                    return &object.String{Value: "Error: first argument must be number"}
                }

                switch arg := args[1].(type) {
                case *object.Integer:
                    b = float64(arg.Value)
                case *object.Float:
                    b = arg.Value
                default:
                    return &object.String{Value: "Error: second argument must be number"}
                }

                return &object.Float{Value: a + b}
            },
        },
        "multiply": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                if len(args) != 2 {
                    return &object.String{Value: "Error: multiply requires 2 arguments"}
                }

                var a, b float64

                switch arg := args[0].(type) {
                case *object.Integer:
                    a = float64(arg.Value)
                case *object.Float:
                    a = arg.Value
                default:
                    return &object.String{Value: "Error: first argument must be number"}
                }

                switch arg := args[1].(type) {
                case *object.Integer:
                    b = float64(arg.Value)
                case *object.Float:
                    b = arg.Value
                default:
                    return &object.String{Value: "Error: second argument must be number"}
                }

                return &object.Float{Value: a * b}
            },
        },
    }
}

// Usage
func main() {
    p := scriptling.New()
    p.RegisterLibrary("mymath", CreateMathLibrary())

    p.Eval(`
import mymath
result = mymath.add(3.14, 2.86)
print(result)  # 6.0
`)
}
```

### Library with State and Output Capture

```go
// Logger library that maintains state and uses output capture
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
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                if len(args) != 1 {
                    return &object.String{Value: "Error: set_level requires 1 argument"}
                }

                levelObj, ok := args[0].(*object.String)
                if !ok {
                    return &object.String{Value: "Error: level must be string"}
                }

                l.level = levelObj.Value
                return &object.String{Value: "Level set to " + l.level}
            },
        },
        "log": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                if len(args) != 1 {
                    return &object.String{Value: "Error: log requires 1 argument"}
                }

                msgObj, ok := args[0].(*object.String)
                if !ok {
                    return &object.String{Value: "Error: message must be string"}
                }

                // Get environment for output
                env := evaluator.GetEnvFromContext(ctx)
                writer := env.GetWriter()

                // Format log message
                logMsg := fmt.Sprintf("[%s] %s", l.level, msgObj.Value)
                l.messages = append(l.messages, logMsg)

                // Output to current writer (stdout or capture buffer)
                fmt.Fprintln(writer, logMsg)

                return &object.String{Value: "logged"}
            },
        },
        "get_messages": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                elements := make([]object.Object, len(l.messages))
                for i, msg := range l.messages {
                    elements[i] = &object.String{Value: msg}
                }
                return &object.List{Elements: elements}
            },
        },
        "clear": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                l.messages = l.messages[:0] // Clear slice
                return &object.String{Value: "Messages cleared"}
            },
        },
    }
}

// Usage with output capture
func main() {
    p := scriptling.New()
    logger := NewLogger()
    p.RegisterLibrary("logger", logger.CreateLibrary())

    // Enable output capture
    p.EnableOutputCapture()

    p.Eval(`
import logger
logger.set_level("DEBUG")
logger.log("Application started")
logger.log("Processing data")
logger.log("Application finished")
`)

    // Get captured output
    output := p.GetOutput()
    fmt.Printf("Captured logs:\n%s", output)

    // Get stored messages
    if messages, ok := p.GetVar("logger"); ok {
        // Access library functions if needed
    }
}
```

## Creating Custom Classes and Instances

Scriptling supports object-oriented programming through custom classes and instances. Classes define the structure and behavior of objects, while instances are concrete objects created from classes.

### Basic Class Creation

```go
package main

import (
    "context"
    "fmt"
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
)

func main() {
    p := scriptling.New()

    // Create a Person class
    personClass := &object.Class{
        Name: "Person",
        Methods: map[string]*object.Builtin{
            "greet": {
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    // 'this' is the instance (first argument)
                    if len(args) < 1 {
                        return &object.Error{Message: "greet requires instance"}
                    }
                    instance := args[0].(*object.Instance)

                    name, _ := instance.Fields["name"].(*object.String)
                    return &object.String{Value: "Hello, my name is " + name.Value}
                },
                HelpText: "Return a greeting message from this person",
            },
            "set_age": {
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    if len(args) < 2 {
                        return &object.Error{Message: "set_age requires instance and age"}
                    }
                    instance := args[0].(*object.Instance)
                    age := args[1]

                    instance.Fields["age"] = age
                    return &object.Null{}
                },
                HelpText: "Set the age of this person",
            },
        },
    }

    // Register the class as a library constant
    p.RegisterLibrary("person", object.NewLibrary(map[string]*object.Builtin{
        "create": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                if len(args) < 1 {
                    return &object.Error{Message: "create requires name"}
                }
                name := args[0].(*object.String)

                // Create instance with initial fields
                instance := &object.Instance{
                    Class: personClass,
                    Fields: map[string]object.Object{
                        "name": name,
                        "age": &object.Integer{Value: 0},
                    },
                }
                return instance
            },
            HelpText: "Create a new Person instance with the given name",
        },
        "Person": personClass, // Expose the class itself for help() and isinstance()
    }, nil, "Person class and factory functions"))

    // Use the class in Scriptling
    p.Eval(`
import person

# Create a person
john = person.create("John")
print(john.greet())  # Hello, my name is John

# Set age and access fields
john.set_age(30)
print("Age: " + str(john.age))  # Age: 30

# Help works on classes and instances
help(person.Person)  # Shows class info with methods
help(john)           # Shows instance info with fields and methods
`)
}
```

### Class with Constructor

```go
func createPersonClass() *object.Class {
    return &object.Class{
        Name: "Person",
        Methods: map[string]*object.Builtin{
            "__init__": {
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    if len(args) < 2 {
                        return &object.Error{Message: "__init__ requires instance, name, and age"}
                    }
                    instance := args[0].(*object.Instance)
                    name := args[1].(*object.String)
                    age := args[2].(*object.Integer)

                    instance.Fields["name"] = name
                    instance.Fields["age"] = age
                    return &object.Null{}
                },
            },
            "introduce": {
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    instance := args[0].(*object.Instance)
                    name, _ := instance.Fields["name"].(*object.String)
                    age, _ := instance.Fields["age"].(*object.Integer)
                    return &object.String{Value: fmt.Sprintf("Hi, I'm %s and I'm %d years old", name.Value, age.Value)}
                },
                HelpText: "Return an introduction string",
            },
        },
    }
}

func main() {
    p := scriptling.New()

    personClass := createPersonClass()

    p.RegisterLibrary("person", object.NewLibrary(map[string]*object.Builtin{
        "Person": personClass,
        "new": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                if len(args) < 2 {
                    return &object.Error{Message: "new requires name and age"}
                }

                // Create instance
                instance := &object.Instance{
                    Class: personClass,
                    Fields: make(map[string]object.Object),
                }

                // Call constructor
                initMethod := personClass.Methods["__init__"]
                initMethod.Fn(ctx, nil, instance, args[0], args[1])

                return instance
            },
            HelpText: "Create a new Person with name and age",
        },
    }, nil, "Person class with constructor"))

    p.Eval(`
import person

alice = person.new("Alice", 25)
print(alice.introduce())  # Hi, I'm Alice and I'm 25 years old
`)
}
```

### Class Inheritance (Composition)

```go
func createEmployeeClass(personClass *object.Class) *object.Class {
    return &object.Class{
        Name: "Employee",
        Methods: map[string]*object.Builtin{
            "__init__": {
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    if len(args) < 4 {
                        return &object.Error{Message: "__init__ requires instance, name, age, department, salary"}
                    }
                    instance := args[0].(*object.Instance)
                    name := args[1].(*object.String)
                    age := args[2].(*object.Integer)
                    department := args[3].(*object.String)
                    salary := args[4].(*object.Float)

                    // Initialize as person
                    personInit := personClass.Methods["__init__"]
                    personInit.Fn(ctx, nil, instance, name, age)

                    // Add employee-specific fields
                    instance.Fields["department"] = department
                    instance.Fields["salary"] = salary
                    return &object.Null{}
                },
            },
            "get_salary_info": {
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    instance := args[0].(*object.Instance)
                    dept, _ := instance.Fields["department"].(*object.String)
                    salary, _ := instance.Fields["salary"].(*object.Float)
                    return &object.String{Value: fmt.Sprintf("Works in %s, earns $%.2f", dept.Value, salary.Value)}
                },
                HelpText: "Return salary and department information",
            },
            // Inherit greet method from person
            "greet": personClass.Methods["greet"],
            "introduce": {
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    instance := args[0].(*object.Instance)

                    // Call parent introduce
                    parentIntro := personClass.Methods["introduce"]
                    baseIntro := parentIntro.Fn(ctx, nil, instance)

                    // Add employee info
                    salaryInfo := instance.Class.Methods["get_salary_info"].Fn(ctx, nil, instance)

                    return &object.String{Value: baseIntro.(*object.String).Value + ". " + salaryInfo.(*object.String).Value}
                },
                HelpText: "Return a complete introduction including employee info",
            },
        },
    }
}
```

### Instance Field Access and Modification

```go
func main() {
    p := scriptling.New()

    counterClass := &object.Class{
        Name: "Counter",
        Methods: map[string]*object.Builtin{
            "__init__": {
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    instance := args[0].(*object.Instance)
                    instance.Fields["count"] = &object.Integer{Value: 0}
                    return &object.Null{}
                },
            },
            "increment": {
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    instance := args[0].(*object.Instance)
                    count := instance.Fields["count"].(*object.Integer)
                    count.Value++
                    return count
                },
                HelpText: "Increment the counter and return new value",
            },
            "get_count": {
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    instance := args[0].(*object.Instance)
                    return instance.Fields["count"]
                },
                HelpText: "Get the current count value",
            },
        },
    }

    p.RegisterLibrary("counter", object.NewLibrary(map[string]*object.Builtin{
        "Counter": counterClass,
        "new": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                instance := &object.Instance{
                    Class: counterClass,
                    Fields: make(map[string]object.Object),
                }
                counterClass.Methods["__init__"].Fn(ctx, nil, instance)
                return instance
            },
            HelpText: "Create a new Counter instance",
        },
    }, nil, "Counter class for counting operations"))

    p.Eval(`
import counter

c = counter.new()
print("Initial: " + str(c.get_count()))  # Initial: 0

c.increment()
c.increment()
print("After increments: " + str(c.get_count()))  # After increments: 2

# Direct field access
c.count = 10
print("After direct assignment: " + str(c.count))  # After direct assignment: 10
`)
}
```

### Best Practices for Classes

1. **Use Constructors**: Always provide an `__init__` method for proper initialization
2. **Document Methods**: Add `HelpText` to all public methods for the help system
3. **Expose Classes**: Register classes as library constants so `help()` and `isinstance()` work
4. **Field Access**: Allow both method-based and direct field access
5. **Error Handling**: Validate arguments and return meaningful error messages
6. **Composition over Inheritance**: Use composition by including other class methods rather than complex inheritance
7. **Type Safety**: Check types when accessing fields and method arguments

### Special Methods for Custom Behavior

Scriptling supports special methods that enable custom syntax and behavior for your classes:

#### `__getitem__(key)` - Custom Indexing

Implement `__getitem__` to enable `obj[key]` syntax for custom indexing:

```go
counterClass := &object.Class{
    Name: "Counter",
    Methods: map[string]*object.Builtin{
        "__init__": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                instance := args[0].(*object.Instance)
                instance.Fields = make(map[string]object.Object)
                return &object.Null{}
            },
        },
        "__getitem__": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                if len(args) != 2 {
                    return &object.Error{Message: "__getitem__ requires instance and key"}
                }
                instance := args[0].(*object.Instance)
                key := args[1].Inspect() // Convert key to string for storage

                if count, ok := instance.Fields[key]; ok {
                    return count
                }
                // Return 0 for missing keys (like Python Counter)
                return &object.Integer{Value: 0}
            },
            HelpText: `__getitem__(key) - Get count for key (supports c[key] syntax)`,
        },
        // ... other methods
    },
}
```

This enables:

```python
c = Counter([1, 1, 2])
print(c[1])  # 2
print(c[3])  # 0 (not KeyError)
```

#### Other Special Methods

- `__init__`: Constructor called when creating instances
- `__str__`: Custom string representation (for `str()` function)
- `__len__`: Custom length (for `len()` function)

### Integration with Help System

When you expose classes in libraries, the help system automatically provides information about:

- Class methods and their documentation
- Instance fields and their current values
- Method signatures and help text

```go
// This enables:
help(my_library.MyClass)    // Shows class info and methods
help(instance)              // Shows instance fields and available methods
isinstance(obj, MyClass)    // Type checking
```

Classes and instances integrate seamlessly with Scriptling's object system and can be used anywhere regular objects are expected.

## Database Library Example

```go
package dblib

import (
    "context"
    "database/sql"
    "fmt"
    _ "github.com/lib/pq" // PostgreSQL driver
)

type Database struct {
    db *sql.DB
}

func NewDatabase(connectionString string) (*Database, error) {
    db, err := sql.Open("postgres", connectionString)
    if err != nil {
        return nil, err
    }
    return &Database{db: db}, nil
}

func (d *Database) CreateLibrary() map[string]*object.Builtin {
    return map[string]*object.Builtin{
        "query": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                if len(args) != 1 {
                    return &object.String{Value: "Error: query requires 1 argument"}
                }

                queryObj, ok := args[0].(*object.String)
                if !ok {
                    return &object.String{Value: "Error: query must be string"}
                }

                // Get environment for logging
                env := evaluator.GetEnvFromContext(ctx)
                writer := env.GetWriter()

                fmt.Fprintf(writer, "Executing query: %s\n", queryObj.Value)

                rows, err := d.db.Query(queryObj.Value)
                if err != nil {
                    fmt.Fprintf(writer, "Query failed: %s\n", err.Error())
                    return &object.String{Value: ""}
                }
                defer rows.Close()

                // Get column names
                columns, err := rows.Columns()
                if err != nil {
                    fmt.Fprintf(writer, "Failed to get columns: %s\n", err.Error())
                    return &object.String{Value: ""}
                }

                // Build result as list of dictionaries
                var results []object.Object

                for rows.Next() {
                    // Create slice to hold column values
                    values := make([]interface{}, len(columns))
                    valuePtrs := make([]interface{}, len(columns))
                    for i := range values {
                        valuePtrs[i] = &values[i]
                    }

                    if err := rows.Scan(valuePtrs...); err != nil {
                        fmt.Fprintf(writer, "Failed to scan row: %s\n", err.Error())
                        continue
                    }

                    // Create dictionary for this row
                    pairs := make(map[string]object.DictPair)
                    for i, col := range columns {
                        var val object.Object
                        if values[i] == nil {
                            val = &object.Null{}
                        } else {
                            val = &object.String{Value: fmt.Sprintf("%v", values[i])}
                        }
                        pairs[col] = object.DictPair{
                            Key:   &object.String{Value: col},
                            Value: val,
                        }
                    }

                    results = append(results, &object.Dict{Pairs: pairs})
                }

                fmt.Fprintf(writer, "Query returned %d rows\n", len(results))
                return &object.List{Elements: results}
            },
        },
        "execute": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                if len(args) != 1 {
                    return &object.String{Value: "Error: execute requires 1 argument"}
                }

                queryObj, ok := args[0].(*object.String)
                if !ok {
                    return &object.String{Value: "Error: query must be string"}
                }

                env := getEnvFromContext(ctx)
                writer := env.GetWriter()

                fmt.Fprintf(writer, "Executing statement: %s\n", queryObj.Value)

                result, err := d.db.Exec(queryObj.Value)
                if err != nil {
                    fmt.Fprintf(writer, "Execute failed: %s\n", err.Error())
                    return &object.Integer{Value: 0}
                }

                rowsAffected, _ := result.RowsAffected()
                fmt.Fprintf(writer, "Rows affected: %d\n", rowsAffected)

                return &object.Integer{Value: rowsAffected}
            },
        },
    }
}
```

## Best Practices

### 1. Error Handling

```go
p.RegisterFunc("safe_divide", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) != 2 {
        return &object.String{Value: "Error: divide requires 2 arguments"}
    }

    // Type checking
    aObj, ok := args[0].(*object.Float)
    if !ok {
        if intObj, ok := args[0].(*object.Integer); ok {
            aObj = &object.Float{Value: float64(intObj.Value)}
        } else {
            return &object.String{Value: "Error: first argument must be number"}
        }
    }

    bObj, ok := args[1].(*object.Float)
    if !ok {
        if intObj, ok := args[1].(*object.Integer); ok {
            bObj = &object.Float{Value: float64(intObj.Value)}
        } else {
            return &object.String{Value: "Error: second argument must be number"}
        }
    }

    // Division by zero check
    if bObj.Value == 0 {
        return &object.String{Value: "Error: division by zero"}
    }

    return &object.Float{Value: aObj.Value / bObj.Value}
})
```

### 2. Use Type-Safe Accessor Methods

**Always use the built-in type-safe accessor methods** instead of creating manual helper functions:

```go
// ✓ RECOMMENDED: Use built-in accessors
p.RegisterFunc("power", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    if len(args) != 2 {
        return errors.NewArgumentError(len(args), 2)
    }

    // AsFloat() automatically handles both Integer and Float types
    base, ok := args[0].AsFloat()
    if !ok {
        return errors.NewTypeError("NUMBER", args[0].Type().String())
    }

    exponent, ok := args[1].AsFloat()
    if !ok {
        return errors.NewTypeError("NUMBER", args[1].Type().String())
    }

    result := math.Pow(base, exponent)
    return &object.Float{Value: result}
})
```

**Why use accessors?**

- `Integer.AsFloat()` automatically converts to `float64` - no manual conversion needed
- `AsDict()` returns `map[string]Object` instead of `map[string]DictPair` - cleaner access
- Consistent `(value, ok)` pattern across all types
- Less code, fewer bugs

See the **Type-Safe Accessor Methods** section for complete details and examples.

### 3. Context Usage

```go
p.RegisterFunc("long_operation", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    env := evaluator.GetEnvFromContext(ctx)
    writer := env.GetWriter()

    fmt.Fprintln(writer, "Starting long operation...")

    // Check for cancellation
    select {
    case <-ctx.Done():
        fmt.Fprintln(writer, "Operation cancelled")
        return &object.String{Value: "cancelled"}
    default:
    }

    // Simulate work with periodic cancellation checks
    for i := 0; i < 10; i++ {
        select {
        case <-ctx.Done():
            fmt.Fprintln(writer, "Operation cancelled during work")
            return &object.String{Value: "cancelled"}
        default:
        }

        time.Sleep(100 * time.Millisecond)
        fmt.Fprintf(writer, "Progress: %d/10\n", i+1)
    }

    fmt.Fprintln(writer, "Operation completed")
    return &object.String{Value: "completed"}
})
```

## Testing Custom Functions

```go
func TestCustomFunction(t *testing.T) {
    p := scriptling.New()
    p.EnableOutputCapture() // Enable to test output

    // Register test function
    p.RegisterFunc("test_func", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
        env := evaluator.GetEnvFromContext(ctx)
        writer := env.GetWriter()
        fmt.Fprintln(writer, "Test function called")
        return &object.String{Value: "success"}
    })

    // Test the function
    result, err := p.Eval(`result = test_func()`)
    if err != nil {
        t.Fatalf("Eval error: %v", err)
    }

    // Check output
    output := p.GetOutput()
    if !strings.Contains(output, "Test function called") {
        t.Errorf("Expected output not found: %s", output)
    }

    // Check return value
    if value, ok := p.GetVar("result"); ok {
        if strObj, ok := value.(*object.String); ok {
            if strObj.Value != "success" {
                t.Errorf("Expected 'success', got '%s'", strObj.Value)
            }
        } else {
            t.Error("Expected string result")
        }
    } else {
        t.Error("Result variable not found")
    }
}
```

## Integration Patterns

### 1. Service Integration

```go
type APIService struct {
    baseURL string
    apiKey  string
}

func (s *APIService) CreateLibrary() map[string]*object.Builtin {
    return map[string]*object.Builtin{
        "get_user": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                // Implementation with output capture for logging
                env := evaluator.GetEnvFromContext(ctx)
                writer := env.GetWriter()

                fmt.Fprintf(writer, "Fetching user from API...\n")
                // API call implementation
                return &object.Dict{} // Return user data
            },
        },
    }
}
```

### 2. Configuration-Driven Libraries

```go
func CreateConfigurableLibrary(config map[string]interface{}) map[string]*object.Builtin {
    return map[string]*object.Builtin{
        "process": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                env := evaluator.GetEnvFromContext(ctx)
                writer := env.GetWriter()

                // Use config values
                if debug, ok := config["debug"].(bool); ok && debug {
                    fmt.Fprintf(writer, "Debug mode enabled\n")
                }

                return &object.String{Value: "processed"}
            },
        },
    }
}
```

## Registering Libraries

You can group related functions and constants into a library using `RegisterLibrary`.

```go
package main

import (
    "context"
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
)

func main() {
    p := scriptling.New()
    stdlib.RegisterAll(p)  // Register standard libraries if needed

    // Create a library
    myLib := object.NewLibrary(map[string]*object.Builtin{
        "add": {
            Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
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
    })

    // Register the library
    p.RegisterLibrary("mylib", myLib)

    // Use in script
    p.Eval(`
import mylib
result = mylib.add(1, 2)
print(result)
`)
}
```

## Registering Classes

You can define classes in Go and register them in Scriptling. A class is an `*object.Class` structure containing methods.

```go
package main

import (
    "context"
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
)

func main() {
    p := scriptling.New()
    stdlib.RegisterAll(p)  // Register standard libraries if needed

    // Define a class
    counterClass := &object.Class{
        Name: "Counter",
        Methods: map[string]object.Object{
            "__init__": &object.Builtin{
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    // Initialize instance
                    if instance, ok := args[0].(*object.Instance); ok {
                        instance.Fields["count"] = &object.Integer{Value: 0}
                    }
                    return object.None
                },
            },
            "increment": &object.Builtin{
                Fn: func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
                    if instance, ok := args[0].(*object.Instance); ok {
                        if count, ok := instance.Fields["count"].(*object.Integer); ok {
                            count.Value++
                            return count
                        }
                    }
                    return object.None
                },
            },
        },
    }

    // Register class as a global variable
    p.SetVar("Counter", counterClass)

    // Or add to a library
    myLib := object.NewLibrary(nil)
    myLib.Set("Counter", counterClass)
    p.RegisterLibrary("mylib", myLib)

    // Use in script
    p.Eval(`
c = Counter()
c.increment()
print(c.increment())
`)
}
```
