# Go Integration Guide

This guide covers how to embed and use Scriptling from Go applications.

## Installation

```bash
go get github.com/paularlott/scriptling
```

## Basic Usage

### Create Interpreter

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

    // Register standard libraries as needed
    stdlib.RegisterAll(p)  // Register all standard libraries
    // Or register individual libraries:
    // p.RegisterLibrary(stdlib.JSONLibraryName, stdlib.JSONLibrary)

    // Execute Scriptling code
    result, err := p.Eval(`x = 5 + 3`)
    if err != nil {
        fmt.Println("Error:", err)
    }
}
```

### Execute Code

```go
// Simple execution
result, err := p.Eval("x = 42")

// Multi-line script
script := `
def fibonacci(n):
    if n <= 1:
        return n
    else:
        return fibonacci(n - 1) + fibonacci(n - 2)

result = fibonacci(10)
`
result, err := p.Eval(script)
```

## Variable Exchange

### Set Variables from Go

```go
    p.Eval(configScript)
    if dbHost, ok := p.GetVarAsString("db_host"); ok {
        fmt.Printf("Database host: %s\n", dbHost)
    }
    if cacheSize, ok := p.GetVarAsInt("cache_size"); ok {
        fmt.Printf("Cache size: %d\n", cacheSize)
    }
```

### Get Variables from Scriptling

```go
// Execute script that sets variables
p.Eval(`
x = 42
name = "Alice"
result = {"status": "success", "count": 10}
`)

// Get variables using convenience methods (recommended)
if value, ok := p.GetVarAsInt("x"); ok {
    fmt.Printf("x = %d\n", value)  // x = 42
}

if name, ok := p.GetVarAsString("name"); ok {
    fmt.Printf("name = %s\n", name)  // name = Alice
}

if count, ok := p.GetVarAsBool("flag"); ok {
    fmt.Printf("flag = %t\n", count)  // flag = true
}

// Get variables using generic GetVar (advanced use cases)
if value, ok := p.GetVar("result"); ok {
    fmt.Printf("result = %v\n", value)  // result = {status: success, count: 10}
}

// Get complex types
if numbers, ok := p.GetVarAsList("numbers"); ok {
    fmt.Printf("First number: %s\n", numbers[0].Inspect())  // Access list elements
}

if config, ok := p.GetVarAsDict("config"); ok {
    if host, ok := config["host"]; ok {
        fmt.Printf("Host: %s\n", host.Inspect())  // Access dict values
    }
}
```

## Script Return Values

Scripts can return values to Go using the last expression evaluated. Use the `Eval()` return value to access these results.

### Basic Return Values

```go
// Script returns a simple value
result, err := p.Eval(`
x = 42
y = 24
x + y  # Last expression becomes return value
`)

if err != nil {
    fmt.Println("Error:", err)
    return
}

// Access the return value
if intResult, err := result.AsInt(); err == nil {
    fmt.Printf("Result: %d\n", intResult)  // Result: 66
}
```

### Complex Return Values

```go
// Script returns a dictionary
result, err := p.Eval(`
data = {"name": "Alice", "age": 30, "active": True}
numbers = [1, 2, 3, 4, 5]

# Return computed result
{
    "user": data,
    "count": len(numbers),
    "sum": sum(numbers),
    "status": "success"
}
`)

if err != nil {
    fmt.Println("Error:", err)
    return
}

// Access dictionary return value
if dict, err := result.AsDict(); err == nil {
    if status, ok := dict["status"]; ok {
        fmt.Printf("Status: %s\n", status.Inspect())  // Status: success
    }
    if count, ok := dict["count"]; ok {
        if countVal, err := count.AsInt(); err == nil {
            fmt.Printf("Count: %d\n", countVal)  // Count: 5
        }
    }
}
```

### Return Value Types

```go
// Different return value types
scripts := []string{
    `42`,                    // Integer
    `"hello"`,              // String
    `3.14`,                 // Float
    `True`,                 // Boolean
    `[1, 2, 3]`,           // List
    `{"key": "value"}`,     // Dictionary
}

for _, script := range scripts {
    result, _ := p.Eval(script)
    fmt.Printf("Script: %s -> Type: %s, Value: %s\n",
        script, result.Type(), result.Inspect())
}
```

### Processing Return Values

```go
// Script processes data and returns result
result, err := p.Eval(`
# Process input data
input = [10, 20, 30, 40, 50]
filtered = [x for x in input if x > 25]
total = sum(filtered)

# Return processed result
{
    "original_count": len(input),
    "filtered": filtered,
    "total": total,
    "average": total / len(filtered)
}
`)

if err != nil {
    fmt.Println("Error:", err)
    return
}

// Process the returned dictionary
if resultDict, err := result.AsDict(); err == nil {
    fmt.Println("Processing Results:")

    if count, ok := resultDict["original_count"]; ok {
        if countVal, err := count.AsInt(); err == nil {
            fmt.Printf("  Original items: %d\n", countVal)
        }
    }

    if total, ok := resultDict["total"]; ok {
        if totalVal, err := total.AsInt(); err == nil {
            fmt.Printf("  Filtered total: %d\n", totalVal)
        }
    }

    if filtered, ok := resultDict["filtered"]; ok {
        if filteredList, err := filtered.AsList(); err == nil {
            fmt.Printf("  Filtered items: %d\n", len(filteredList))
        }
    }
}
```

## Extending Scriptling with Go

This guide focuses on **using** Scriptling from Go. For information on **extending** Scriptling by registering custom Go functions, libraries, and classes, see [EXTENDING_WITH_GO.md](EXTENDING_WITH_GO.md).

## Call Functions Directly from Go

Instead of writing script strings to call functions, you can call registered and script-defined functions directly with Go arguments using `CallFunction()`:

### Calling Registered Functions

```go
p.RegisterFunc("multiply", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    a, _ := args[0].AsInt()
    b, _ := args[1].AsInt()
    return &object.Integer{Value: a * b}
})

// Call with Go arguments - no script string needed!
result, err := p.CallFunction("multiply", 6, 7)
if err != nil {
    log.Fatal(err)
}

product, _ := result.AsInt()
fmt.Printf("Product: %d\n", product)  // Product: 42
```

### Calling Script-Defined Functions

```go
// Define a function in script
p.Eval(`
def greet(name):
    return 'Hello, ' + name
`)

// Call it directly from Go
result, err := p.CallFunction("greet", "World")
if err != nil {
    log.Fatal(err)
}

message, _ := result.AsString()
fmt.Printf("Message: %s\n", message)  // Message: Hello, World
```

### Type Conversions

`CallFunction` automatically converts Go types to Scriptling objects:

| Go Type | Scriptling Type |
|---------|-----------------|
| `int`, `int64` | Integer |
| `float64` | Float |
| `string` | String |
| `bool` | Boolean |
| `[]T` | List |
| `map[string]T` | Dict |

### Return Values

`CallFunction` returns an `object.Object` - use type assertion methods to extract values:

```go
result, err := p.CallFunction("some_function", arg1, arg2)

// Extract the value based on expected type
if i, err := result.AsInt(); err == nil {
    fmt.Printf("Integer: %d\n", i)
} else if s, err := result.AsString(); err == nil {
    fmt.Printf("String: %s\n", s)
} else if f, err := result.AsFloat(); err == nil {
    fmt.Printf("Float: %f\n", f)
}
```

### Error Handling

```go
result, err := p.CallFunction("my_function", arg1, arg2)
if err != nil {
    // Function not found or execution error
    fmt.Printf("Error: %v\n", err)
    return
}

// Check if result is an error object
if errObj, ok := result.(*object.Error); ok {
    fmt.Printf("Function returned error: %s\n", errObj.Message)
    return
}
```

### Using Context

For timeout or cancellation control, use `CallFunctionWithContext`:

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := p.CallFunctionWithContext(ctx, "slow_operation", data)
if err != nil {
    log.Fatal(err)
}
```

### Using Keyword Arguments

Pass keyword arguments as the last parameter using `map[string]interface{}`:

```go
// Register a function with keyword arguments
p.RegisterFunc("format", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    text, _ := args[0].AsString()
    prefix := kwargs.MustGetString("prefix", "")
    suffix := kwargs.MustGetString("suffix", "")
    return &object.String{Value: prefix + text + suffix}
})

// Call with keyword arguments
result, err := p.CallFunction("format", "hello",
    map[string]interface{}{
        "prefix": ">> ",
        "suffix": " <<",
    })
if err != nil {
    log.Fatal(err)
}

message, _ := result.AsString()
fmt.Println(message)  // >> hello <<
```

#### Keyword Argument Types

```go
// Register function with multiple kwarg types
p.RegisterFunc("configure", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    enabled := kwargs.MustGetBool("enabled", false)
    count := kwargs.MustGetInt("count", 0)
    rate := kwargs.MustGetFloat("rate", 1.0)
    name := kwargs.MustGetString("name", "default")
    // ... use the values
    return &object.Null{}
})

// Call with mixed type kwargs
result, err := p.CallFunction("configure", nil,
    map[string]interface{}{
        "enabled": true,
        "count":   42,
        "rate":    3.14,
        "name":    "example",
    })
```

#### Script Functions with Kwargs

```go
// Define a script function with default keyword arguments
p.Eval(`
def greet(name, greeting="Hello", punctuation="!"):
    return greeting + ", " + name + punctuation
`)

// Call with positional args only
result, _ := p.CallFunction("greet", "World")
text, _ := result.AsString()
fmt.Println(text)  // Hello, World!

// Call with keyword arguments
result, _ = p.CallFunction("greet", "Alice",
    map[string]interface{}{
        "greeting": "Hi",
        "punctuation": "?",
    })
text, _ = result.AsString()
fmt.Println(text)  // Hi, Alice?
```

## Programmatic Library Import

Instead of using `import` statements in scripts, you can import libraries programmatically from Go:

```go
// Import libraries before executing scripts
p.Import("json")
p.Import("math")

// Now use libraries in scripts without import statements
p.Eval(`
data = json.dumps({"numbers": [1, 2, 3]})
result = math.sqrt(16)  # 4.0
`)
```

This is useful when you want to pre-load commonly used libraries or control which libraries are available.

## Complete Integration Example

```go
package main

import (
    "fmt"
    "log"

    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/stdlib"
    "github.com/paularlott/scriptling/extlibs"
)

func main() {
    // Create interpreter
    p := scriptling.New()

    // Register libraries
    stdlib.RegisterAll(p)
    p.RegisterLibrary(extlibs.RequestsLibraryName, extlibs.RequestsLibrary)
    extlibs.RegisterOSLibrary(p, []string{"/tmp"})
    extlibs.RegisterPathlibLibrary(p, []string{"/tmp"})

    // Set configuration from Go
    p.SetVar("api_base", "https://api.example.com")
    p.SetVar("timeout", 30)

    // Execute automation script
    script := `
import json, requests

# Fetch data
url = api_base + "/users"
options = {"timeout": timeout}
response = requests.get(url, options)

if response["status"] == 200:
    users = json.parse(response["body"])
    print("Found " + str(len(users)) + " users")

    # Process each user
    processed_count = 0
    for user in users:
        if user["active"]:
            print("Processing user: " + user["name"])
            processed_count = processed_count + 1
            # Additional processing...

    success = True
else:
    print("API call failed: " + str(response["status"]))
    processed_count = 0
    success = False

# Return summary
{
    "success": success,
    "total_users": len(users) if "users" in locals() else 0,
    "processed_count": processed_count,
    "api_status": response["status"]
}
`

    result, err := p.Eval(script)
    if err != nil {
        log.Fatalf("Script error: %v", err)
    }

    // Access return value from script
    if resultDict, err := result.AsDict(); err == nil {
        if success, ok := resultDict["success"]; ok {
            if successVal, err := success.AsBool(); err == nil {
                fmt.Printf("Automation completed: %t\n", successVal)
            }
        }
        if processed, ok := resultDict["processed_count"]; ok {
            if count, err := processed.AsInt(); err == nil {
                fmt.Printf("Processed %d items\n", count)
            }
        }
    }

    fmt.Printf("Script result: %s\n", result.Inspect())
}
}
```

## Object Types

When working with Scriptling objects in Go:

### String Objects
```go
strObj := &object.String{Value: "hello"}
if str, ok := obj.(*object.String); ok {
    value := str.Value  // "hello"
}
```

### Integer Objects
```go
intObj := &object.Integer{Value: 42}
if integer, ok := obj.(*object.Integer); ok {
    value := integer.Value  // 42
}
```

### Boolean Objects
```go
boolObj := &object.Boolean{Value: true}
if boolean, ok := obj.(*object.Boolean); ok {
    value := boolean.Value  // true
}
```

### Float Objects
```go
floatObj := &object.Float{Value: 3.14}
if float, ok := obj.(*object.Float); ok {
    value := float.Value  // 3.14
}
```

## Output Capture

By default, the `print()` function outputs to stdout. You can capture this output programmatically:

### Default Behavior (stdout)
```go
p := scriptling.New()
p.Eval(`print("Hello World")`)  // Prints to stdout
```

### Capture Output
```go
p := scriptling.New()
p.EnableOutputCapture()  // Enable output capture

p.Eval(`
print("Line 1")
print("Line 2")
print("Result:", 42)
`)

// Get captured output
output := p.GetOutput()  // Returns "Line 1\nLine 2\nResult: 42\n"
fmt.Print(output)

// Buffer is cleared after GetOutput()
output2 := p.GetOutput()  // Returns ""
```

### Mixed Usage
```go
p := scriptling.New()

// Normal stdout output
p.Eval(`print("This goes to stdout")`)

// Enable capture for specific operations
p.EnableOutputCapture()
p.Eval(`print("This is captured")`)
captured := p.GetOutput()

// Disable capture (output goes back to stdout)
// Note: Currently no disable method - create new instance if needed
```

### Use Cases
- **Testing**: Capture output for assertions
- **Logging**: Redirect script output to custom loggers
- **Processing**: Capture output for further processing
- **UI Integration**: Display script output in applications

```go
// Testing example
func TestScriptOutput(t *testing.T) {
    p := scriptling.New()
    p.EnableOutputCapture()

    p.Eval(`print("test result:", 42)`)
    output := p.GetOutput()

    expected := "test result: 42\n"
    if output != expected {
        t.Errorf("Expected %q, got %q", expected, output)
    }
}
```

### Output Capture in Custom Functions

Custom Go functions can also use the output capture system:

```go
import (
    "context"
    "fmt"
    "github.com/paularlott/scriptling/evaluator"
)

p.RegisterFunc("log_debug", func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object {
    // Get environment from context
    env := evaluator.GetEnvFromContext(ctx)
    writer := env.GetWriter()

    // Write to current output (stdout or capture buffer)
    for _, arg := range args {
        fmt.Fprintf(writer, "[DEBUG] %s\n", arg.Inspect())
    }

    return &object.String{Value: "logged"}
})

// Usage with output capture
p.EnableOutputCapture()
p.Eval(`log_debug("Starting process", 42)`)
output := p.GetOutput() // Contains "[DEBUG] Starting process\n[DEBUG] 42\n"
```

## Error Handling

```go
// Always check for errors
result, err := p.Eval(script)
if err != nil {
    // Handle syntax errors, runtime errors, etc.
    fmt.Printf("Scriptling error: %v\n", err)
    return
}

// Check if variable exists before using
if value, ok := p.GetVar("result"); ok {
    // Variable exists, use value
    fmt.Printf("Result: %v\n", value)
} else {
    // Variable doesn't exist
    fmt.Println("Variable 'result' not found")
}
```

## Performance Tips

1. **Reuse Interpreters**: Create once, use multiple times
2. **Load Only Needed Libraries**: Don't load JSON/HTTP if not needed
3. **Batch Operations**: Execute larger scripts rather than many small ones
4. **Pre-register Functions**: Register all Go functions before execution

```go
// Good: Reuse interpreter
p := scriptling.New()
for _, script := range scripts {
    p.Eval(script)
}

// Bad: Create new interpreter each time
for _, script := range scripts {
    p := scriptling.New()
    p.Eval(script)
}
```

## Testing Integration

```go
func TestScriptlingIntegration(t *testing.T) {
    p := scriptling.New()

    // Test variable setting
    p.SetVar("test_var", 42)

    result, err := p.Eval(`result = test_var * 2`)
    if err != nil {
        t.Fatalf("Eval error: %v", err)
    }

    // Test variable getting with convenience methods
    if result, ok := p.GetVarAsInt("result"); ok {
        if result != 84 {
            t.Errorf("Expected 84, got %d", result)
        }
    } else {
        t.Error("Variable 'result' not found")
    }
}
```

## Best Practices

1. **Always check errors** from `Eval()`
2. **Validate arguments** in custom functions
3. **Use appropriate object types** for return values
4. **Handle missing variables** gracefully with `GetVar()`
5. **Register functions before** executing scripts that use them
6. **Use libraries selectively** - only load what you need
7. **Reuse interpreters** for better performance
8. **Test integration code** thoroughly

## Common Patterns

### Configuration Scripts
```go
import (
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/stdlib"
)

// Load configuration via Scriptling
p := scriptling.New()
stdlib.RegisterAll(p)  // Or register only needed libraries
p.SetVar("env", "production")

configScript := `
if env == "production":
    db_host = "prod.db.example.com"
    cache_size = 1000
else:
    db_host = "dev.db.example.com"
    cache_size = 100
`

p.Eval(configScript)
if dbHost, ok := p.GetVarAsString("db_host"); ok {
    fmt.Printf("Database host: %s\n", dbHost)
}
if cacheSize, ok := p.GetVarAsInt("cache_size"); ok {
    fmt.Printf("Cache size: %d\n", cacheSize)
}
```

### Data Processing Pipeline
```go
import (
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/stdlib"
)

// Process data with Scriptling
p := scriptling.New()
p.RegisterLibrary(stdlib.JSONLibraryName, stdlib.JSONLibrary)
p.SetVar("raw_data", jsonString)

pipeline := `
include json

data = json.parse(raw_data)
processed = []

for item in data:
    if item["active"]:
        processed = append(processed, item["name"])

result = json.stringify(processed)
`

p.Eval(pipeline)
result, _ := p.GetVar("result")
```
## Defining Classes in Go

For information on defining Scriptling classes in Go, including creating custom types with high-performance methods, see [EXTENDING_WITH_GO.md](EXTENDING_WITH_GO.md#defining-classes-in-go).
