# Extending Scriptling with Go

This guide provides an overview of how to extend Scriptling by registering custom Go functions, libraries, and classes.

## Overview

Scriptling is designed to be easily extended from Go. You can add:

- **Functions**: Custom Go functions that can be called from Scriptling
- **Libraries**: Groups of related functions and constants
- **Classes**: Custom types with methods and state

## Two Approaches

Scriptling provides two ways to extend functionality:

| Approach | Pros | Cons | Best For |
|----------|------|------|----------|
| **Native API** | Faster runtime, more control | More verbose, manual type handling | Performance-critical code, complex logic |
| **Builder API** | Cleaner syntax, auto type conversion | Slight overhead, less control | Rapid development, simpler functions |

### Native API

The native API uses the standard Scriptling function signature:

```go
p.RegisterFunc("add", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Manual type checking and conversion
    a, _ := args[0].(*object.Integer)
    b, _ := args[1].(*object.Integer)
    return &object.Integer{Value: a.Value + b.Value}
})
```

### Builder API

The Builder API provides type-safe functions with automatic conversion:

```go
fb := object.NewFunctionBuilder()
fb.Function(func(a, b int) int {
    return a + b
})
p.RegisterFunc("add", fb.Build())
```

## Guides by Topic

### Functions

For detailed information on creating functions, see:

- **[EXTENDING_FUNCTIONS.md](EXTENDING_FUNCTIONS.md)** - Complete guide to adding functions
  - Native API with kwargs, type-safe accessors, output capture
  - Builder API with typed parameters
  - Help text and documentation

### Libraries

For detailed information on creating libraries, see:

- **[EXTENDING_LIBRARIES.md](EXTENDING_LIBRARIES.md)** - Complete guide to creating libraries
  - Native libraries with state
  - Builder API with typed functions
  - Sub-libraries and organization

### Classes

For detailed information on creating classes, see:

- **[EXTENDING_CLASSES.md](EXTENDING_CLASSES.md)** - Complete guide to defining classes
  - Native class creation with inheritance
  - Builder API with type-safe methods
  - Special methods and best practices

## Common Concepts

### Function Signature

All Scriptling functions use a unified signature:

```go
func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object
```

- `ctx`: Context for cancellation and environment access
- `kwargs`: Keyword arguments wrapper with helper methods
- `args`: Positional arguments as Scriptling objects
- Returns: A Scriptling object result

### Kwargs Helper Methods

The `object.Kwargs` type provides convenient methods for extracting keyword arguments:

| Method | Description |
|--------|-------------|
| `GetString(name, default) (string, error)` | Extract string with default |
| `GetInt(name, default) (int64, error)` | Extract int (accepts Integer/Float) |
| `GetFloat(name, default) (float64, error)` | Extract float |
| `GetBool(name, default) (bool, error)` | Extract bool |
| `Has(name) bool` | Check if key exists |

#### Must* Variants

For simple cases where you want to use defaults on any error:

| Method | Description |
|--------|-------------|
| `MustGetString(name, default) string` | Extract string, ignore errors |
| `MustGetInt(name, default) int64` | Extract int, ignore errors |
| `MustGetFloat(name, default) float64` | Extract float, ignore errors |
| `MustGetBool(name, default) bool` | Extract bool, ignore errors |

### Type-Safe Accessor Methods

All Scriptling objects implement type-safe accessor methods:

```go
// Instead of manual type assertions
if intObj, ok := args[0].(*object.Integer); ok {
    value := intObj.Value
}

// Use accessor methods (cleaner, auto-converts)
value, err := args[0].AsInt()  // Works on Integer and Float
```

| Method | Description |
|--------|-------------|
| `AsString() (string, error)` | Extract string value |
| `AsInt() (int64, error)` | Extract integer (floats truncate) |
| `AsFloat() (float64, error)` | Extract float (ints convert) |
| `AsBool() (bool, error)` | Extract boolean |
| `AsList() ([]Object, error)` | Extract list elements (returns a copy) |
| `AsDict() (map[string]Object, error)` | Extract dict as map (keys are human-readable strings) |

### Creating Dict Objects from Go

Use `NewStringDict` to create dicts with string keys:

```go
// Create a dict with string keys
result := object.NewStringDict(map[string]object.Object{
    "name":  &object.String{Value: "Alice"},
    "age":   &object.Integer{Value: 30},
})

// Access and modify using convenience methods
if pair, ok := result.GetByString("name"); ok {
    fmt.Println(pair.Value.Inspect()) // "Alice"
}

result.SetByString("email", &object.String{Value: "alice@example.com"})
result.HasByString("name")       // true
result.DeleteByString("email")   // remove key
```

**Note:** Never create dicts by directly manipulating `Pairs` map keys. The internal key format uses type-prefixed canonical keys (`DictKey`). Always use `NewStringDict`, `SetByString`, or `GetByString` for safe access from Go code.

### Output Capture

Functions can access the output capture system:

```go
import "github.com/paularlott/scriptling/evaluator"

p.RegisterFunc("log", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    env := evaluator.GetEnvFromContext(ctx)
    writer := env.GetWriter()
    fmt.Fprintln(writer, "Log:", args[0].Inspect())
    return object.NULL
})
```

## Best Practices

### 1. Error Handling

Always validate arguments and return meaningful errors:

```go
p.RegisterFunc("divide", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    if len(args) != 2 {
        return &object.Error{Message: "divide requires 2 arguments"}
    }
    // ... rest of implementation
})
```

### 2. Use Type-Safe Accessors

Prefer `AsInt()`, `AsString()`, etc. over manual type assertions:

```go
// Good: Clean and handles type coercion
value, err := args[0].AsInt()
if err != nil {
    return &object.Error{Message: err.Error()}
}

// Avoid: Manual type assertions
if intObj, ok := args[0].(*object.Integer); ok {
    value := intObj.Value
} else if floatObj, ok := args[0].(*object.Float); ok {
    value := int64(floatObj.Value)
}
```

### 3. Add Help Text

Provide documentation for discoverability:

```go
p.RegisterFunc("calculate", func(...) object.Object {
    // implementation
}, `calculate(x, y) - Perform calculation

  Parameters:
    x - First number
    y - Second number

  Returns:
    The calculated result`)
```

### 4. Context Usage

Respect context cancellation for long-running operations:

```go
p.RegisterFunc("long_operation", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    select {
    case <-ctx.Done():
        return &object.Error{Message: "operation cancelled"}
    case <-time.After(5 * time.Second):
        return &object.String{Value: "complete"}
    }
})
```

## Testing Extensions

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

## Integration Patterns

### Service Integration

```go
type APIService struct {
    baseURL string
    apiKey  string
}

func (s *APIService) CreateLibrary() map[string]*object.Builtin {
    return map[string]*object.Builtin{
        "get_user": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                // Use s.baseURL and s.apiKey
                return object.NewStringDict(map[string]object.Object{})
            },
        },
    }
}
```

### Configuration-Driven Libraries

```go
func CreateConfigurableLibrary(config map[string]interface{}) map[string]*object.Builtin {
    debugMode := config["debug"].(bool)
    return map[string]*object.Builtin{
        "process": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
                if debugMode {
                    fmt.Println("Debug mode enabled")
                }
                return &object.String{Value: "processed"}
            },
        },
    }
}
```

## Quick Reference

| Task | Native API | Builder API |
|------|------------|-------------|
| Add function | `p.RegisterFunc(name, fn)` | `fb := NewFunctionBuilder(); fb.Function(...)` |
| Add library | `p.RegisterLibrary( lib)` | `lb := NewLibraryBuilder(name); ...` |
| Add class | `p.SetVar(name, class)` | `cb := NewClassBuilder(name); ...` |
| Get kwargs | `kwargs.GetString(name, default)` | `kwargs.GetString(name, default)` (same) |
| Type convert | `obj.AsInt(), obj.AsString()` | Automatic with typed params |

## See Also

- **[EXTENDING_FUNCTIONS.md](EXTENDING_FUNCTIONS.md)** - Detailed function creation guide
- **[EXTENDING_LIBRARIES.md](EXTENDING_LIBRARIES.md)** - Detailed library creation guide
- **[EXTENDING_CLASSES.md](EXTENDING_CLASSES.md)** - Detailed class definition guide
- **[EXTENDING_WITH_SCRIPTS.md](EXTENDING_WITH_SCRIPTS.md)** - Creating extensions in Scriptling
- **[GO_INTEGRATION.md](GO_INTEGRATION.md)** - Using Scriptling from Go
- **[HELP_SYSTEM.md](HELP_SYSTEM.md)** - Adding documentation
