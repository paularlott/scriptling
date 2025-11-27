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
)

func main() {
    // Create interpreter without standard libraries
    p := scriptling.New()

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
if intResult, ok := result.AsInt(); ok {
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
if dict, ok := result.AsDict(); ok {
    if status, ok := dict["status"]; ok {
        fmt.Printf("Status: %s\n", status.Inspect())  // Status: success
    }
    if count, ok := dict["count"]; ok {
        if countVal, ok := count.AsInt(); ok {
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
if resultDict, ok := result.AsDict(); ok {
    fmt.Println("Processing Results:")

    if count, ok := resultDict["original_count"]; ok {
        if countVal, ok := count.AsInt(); ok {
            fmt.Printf("  Original items: %d\n", countVal)
        }
    }

    if total, ok := resultDict["total"]; ok {
        if totalVal, ok := total.AsInt(); ok {
            fmt.Printf("  Filtered total: %d\n", totalVal)
        }
    }

    if filtered, ok := resultDict["filtered"]; ok {
        if filteredList, ok := filtered.AsList(); ok {
            fmt.Printf("  Filtered items: %d\n", len(filteredList))
        }
    }
}
```

## Register Go Functions

### Basic Function Registration

```go
// Register a simple function
p.RegisterFunc("multiply", func(ctx context.Context, args ...object.Object) object.Object {
    if len(args) != 2 {
        return &object.String{Value: "multiply requires 2 arguments"}
    }

    var a, b int64
    if intObj, ok := args[0].(*object.Integer); ok {
        a = intObj.Value
    }
    if intObj, ok := args[1].(*object.Integer); ok {
        b = intObj.Value
    }

    return &object.Integer{Value: a * b}
})

// Use from Scriptling
p.Eval(`
result = multiply(6, 7)
print(result)  # 42
`)
```

### Advanced Function with Type Checking

```go
import "github.com/paularlott/scriptling/object"

p.RegisterFunc("process_data", func(ctx context.Context, args ...object.Object) object.Object {
    if len(args) != 2 {
        return &object.String{Value: "Error: requires 2 arguments"}
    }

    // Get string argument
    var text string
    if strObj, ok := args[0].(*object.String); ok {
        text = strObj.Value
    } else {
        return &object.String{Value: "Error: first argument must be string"}
    }

    // Get integer argument
    var count int64
    if intObj, ok := args[1].(*object.Integer); ok {
        count = intObj.Value
    } else {
        return &object.String{Value: "Error: second argument must be integer"}
    }

    // Process in Go
    result := strings.Repeat(text, int(count))

    return &object.String{Value: result}
})
```

### Function Returning Different Types

```go
// Function that returns a dictionary
p.RegisterFunc("get_system_info", func(ctx context.Context, args ...object.Object) object.Object {
    pairs := []object.HashPair{
        {
            Key:   &object.String{Value: "os"},
            Value: &object.String{Value: runtime.GOOS},
        },
        {
            Key:   &object.String{Value: "arch"},
            Value: &object.String{Value: runtime.GOARCH},
        },
        {
            Key:   &object.String{Value: "cpus"},
            Value: &object.Integer{Value: int64(runtime.NumCPU())},
        },
    }

    hash := &object.Hash{Pairs: make(map[object.HashKey]object.HashPair)}
    for _, pair := range pairs {
        key := pair.Key.(object.Hashable).HashKey()
        hash.Pairs[key] = pair
    }

    return hash
})

// Use from Scriptling
p.Eval(`
info = get_system_info()
print("OS: " + info["os"])
print("CPUs: " + str(info["cpus"]))
`)
```

## Custom Libraries

### Register Custom Library

```go
// Create library with functions
myLib := object.NewLibrary(map[string]*object.Builtin{
    "hello": {
        Fn: func(ctx context.Context, args ...object.Object) object.Object {
            return &object.String{Value: "Hello from custom library!"}
        },
        HelpText: "hello() - Returns a greeting message",
    },
    "add": {
        Fn: func(ctx context.Context, args ...object.Object) object.Object {
            if len(args) != 2 {
                return &object.Integer{Value: 0}
            }

            var a, b int64
            if intObj, ok := args[0].(*object.Integer); ok {
                a = intObj.Value
            }
            if intObj, ok := args[1].(*object.Integer); ok {
                b = intObj.Value
            }

            return &object.Integer{Value: a + b}
        },
        HelpText: "add(a, b) - Add two integers",
    },
})

// Register the library
p.RegisterLibrary("mylib", myLib)

// Use from Scriptling
p.Eval(`
import mylib
message = mylib.hello()
result = mylib.add(5, 3)
print(message)  # Hello from custom library!
print(result)   # 8

# Alternative bracket notation
result2 = mylib["add"](10, 20)
`)
```

### Library with Constants

```go
// Create library with both functions and constants
mathLib := object.NewLibraryWithConstants(
    map[string]*object.Builtin{
        "sqrt": {
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
                if len(args) != 1 {
                    return &object.Error{Message: "sqrt requires 1 argument"}
                }
                if num, ok := args[0].(*object.Float); ok {
                    return &object.Float{Value: math.Sqrt(num.Value)}
                }
                return &object.Error{Message: "argument must be float"}
            },
            HelpText: "sqrt(x) - Return the square root of x",
        },
    },
    map[string]object.Object{
        "pi": &object.Float{Value: 3.141592653589793},
        "e":  &object.Float{Value: 2.718281828459045},
    },
)

// For a library with functions, constants, and description:
fullLib := object.NewLibraryFull(
    map[string]*object.Builtin{...},  // functions
    map[string]object.Object{...},    // constants
    "Description of the library",
)

p.RegisterLibrary("mymath", mathLib)

// Use constants directly (not as function calls)
p.Eval(`
import mymath
area = mymath.pi * mymath.sqrt(radius)  # pi is a constant, not a function
`)
```

### Library with State

```go
// Library that maintains state
type Counter struct {
    value int64
}

func (c *Counter) CreateLibrary() *object.Library {
    return object.NewLibrary(map[string]*object.Builtin{
        "increment": {
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
                c.value++
                return &object.Integer{Value: c.value}
            },
        },
        "decrement": {
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
                c.value--
                return &object.Integer{Value: c.value}
            },
        },
        "get": {
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
                return &object.Integer{Value: c.value}
            },
        },
        "set": {
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
                if len(args) == 1 {
                    if intObj, ok := args[0].(*object.Integer); ok {
                        c.value = intObj.Value
                    }
                }
                return &object.Integer{Value: c.value}
            },
        },
    })
}

// Usage
counter := &Counter{value: 0}
p.RegisterLibrary("counter", counter.CreateLibrary())

p.Eval(`
import counter
counter.set(10)
print(counter.increment())  # 11
print(counter.increment())  # 12
print(counter.get())        # 12
`)
```

### On-Demand Library Loading

For dynamic library loading from disk or other sources, use the on-demand callback:

```go
// Set callback for loading libraries on-demand
p.SetOnDemandLibraryCallback(func(p *Scriptling, libName string) bool {
    // Check if we can provide this library
    if libName == "mylib" {
        // Load from disk
        script, err := loadLibraryFromFile(libName + ".py")
        if err != nil {
            return false
        }

        // Register the loaded library
        return p.RegisterScriptLibrary(libName, script) == nil
    }

    // Try loading from a plugin system
    if pluginLib := loadFromPluginSystem(libName); pluginLib != nil {
        return p.RegisterLibrary(libName, pluginLib) == nil
    }

    return false // Could not load library
})

// Now scripts can import libraries that don't exist yet
p.Eval(`
import mylib  # Loaded on-demand from disk
result = mylib.do_something()
`)
```

The callback receives the Scriptling instance and library name, and should return `true` if it successfully registered the library.

## Complete Integration Example

```go
package main

import (
    "fmt"
    "log"
    "os"

    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
)

func main() {
    // Create interpreter
    p := scriptling.New()

    // Set configuration from Go
    p.SetVar("api_base", "https://api.example.com")
    p.SetVar("api_key", os.Getenv("API_KEY"))
    p.SetVar("timeout", 30)

    // Register custom logging function
    p.RegisterFunc("log_info", func(ctx context.Context, args ...object.Object) object.Object {
        if len(args) > 0 {
            if strObj, ok := args[0].(*object.String); ok {
                log.Printf("INFO: %s", strObj.Value)
            }
        }
        return &object.String{Value: "logged"}
    })

    // Execute automation script
    script := `
import json, request

log_info("Starting API automation")

# Fetch data
url = api_base + "/users"
options = {"timeout": timeout}
response = requests.get(url, options)

if response["status"] == 200:
    users = json.parse(response["body"])
    log_info("Found " + str(len(users)) + " users")

    # Process each user
    processed_count = 0
    for user in users:
        if user["active"]:
            log_info("Processing user: " + user["name"])
            processed_count = processed_count + 1
            # Additional processing...

    success = True
else:
    log_info("API call failed: " + str(response["status"]))
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

    // Get results using convenience methods
    if success, ok := p.GetVarAsBool("success"); ok {
        fmt.Printf("Automation completed successfully: %t\n", success)
    }

    // Access return value from script
    if resultDict, ok := result.AsDict(); ok {
        if processed, ok := resultDict["processed_count"]; ok {
            if count, ok := processed.AsInt(); ok {
                fmt.Printf("Processed %d items\n", count)
            }
        }
    }

    fmt.Printf("Script result: %s\n", result.Inspect())
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

p.RegisterFunc("log_debug", func(ctx context.Context, args ...object.Object) object.Object {
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
// Load configuration via Scriptling
p := scriptling.New()
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
// Process data with Scriptling
p := scriptling.New()
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