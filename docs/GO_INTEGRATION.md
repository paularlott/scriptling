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
    // Create interpreter without libraries (lightweight)
    p := scriptling.New()

    // Create interpreter with specific libraries
    p := scriptling.New("json")
    p := scriptling.New("json", "http")

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
// Set different types
p.SetVar("api_key", "secret123")
p.SetVar("timeout", 30)
p.SetVar("enabled", true)
p.SetVar("rate", 3.14)

// Use in Scriptling
p.Eval(`
options = {"timeout": timeout}
response = http.get("https://api.example.com/data", options)
if enabled:
    print("API key: " + api_key)
`)
```

### Get Variables from Scriptling

```go
// Execute script that sets variables
p.Eval(`
x = 42
name = "Alice"
result = {"status": "success", "count": 10}
`)

// Get variables in Go
if value, ok := p.GetVar("x"); ok {
    fmt.Printf("x = %v\n", value)  // x = 42
}

if value, ok := p.GetVar("name"); ok {
    fmt.Printf("name = %v\n", value)  // name = Alice
}

if value, ok := p.GetVar("result"); ok {
    fmt.Printf("result = %v\n", value)  // result = {status: success, count: 10}
}
```

## Register Go Functions

### Basic Function Registration

```go
// Register a simple function
p.RegisterFunc("multiply", func(args ...object.Object) object.Object {
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

p.RegisterFunc("process_data", func(args ...object.Object) object.Object {
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
p.RegisterFunc("get_system_info", func(args ...object.Object) object.Object {
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
// Create library functions
myLib := map[string]*object.Builtin{
    "hello": {
        Fn: func(args ...object.Object) object.Object {
            return &object.String{Value: "Hello from custom library!"}
        },
    },
    "add": {
        Fn: func(args ...object.Object) object.Object {
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
    },
}

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

### Library with State

```go
// Library that maintains state
type Counter struct {
    value int64
}

func (c *Counter) CreateLibrary() map[string]*object.Builtin {
    return map[string]*object.Builtin{
        "increment": {
            Fn: func(args ...object.Object) object.Object {
                c.value++
                return &object.Integer{Value: c.value}
            },
        },
        "decrement": {
            Fn: func(args ...object.Object) object.Object {
                c.value--
                return &object.Integer{Value: c.value}
            },
        },
        "get": {
            Fn: func(args ...object.Object) object.Object {
                return &object.Integer{Value: c.value}
            },
        },
        "set": {
            Fn: func(args ...object.Object) object.Object {
                if len(args) == 1 {
                    if intObj, ok := args[0].(*object.Integer); ok {
                        c.value = intObj.Value
                    }
                }
                return &object.Integer{Value: c.value}
            },
        },
    }
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
    // Create interpreter with libraries
    p := scriptling.New("json", "http")

    // Set configuration from Go
    p.SetVar("api_base", "https://api.example.com")
    p.SetVar("api_key", os.Getenv("API_KEY"))
    p.SetVar("timeout", 30)

    // Register custom logging function
    p.RegisterFunc("log_info", func(args ...object.Object) object.Object {
        if len(args) > 0 {
            if strObj, ok := args[0].(*object.String); ok {
                log.Printf("INFO: %s", strObj.Value)
            }
        }
        return &object.String{Value: "logged"}
    })

    // Execute automation script
    script := `
log_info("Starting API automation")

# Fetch data
url = api_base + "/users"
options = {"timeout": timeout}
response = http.get(url, options)

if response["status"] == 200:
    users = json.parse(response["body"])
    log_info("Found " + str(len(users)) + " users")

    # Process each user
    for user in users:
        if user["active"]:
            log_info("Processing user: " + user["name"])
            # Additional processing...

    success = True
else:
    log_info("API call failed: " + str(response["status"]))
    success = False
`

    result, err := p.Eval(script)
    if err != nil {
        log.Fatalf("Script error: %v", err)
    }

    // Get results
    if success, ok := p.GetVar("success"); ok {
        fmt.Printf("Automation completed successfully: %v\n", success)
    }

    fmt.Printf("Script result: %v\n", result.Inspect())
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
p := scriptling.New("json")
for _, script := range scripts {
    p.Eval(script)
}

// Bad: Create new interpreter each time
for _, script := range scripts {
    p := scriptling.New("json")
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

    // Test variable getting
    if value, ok := p.GetVar("result"); ok {
        if intObj, ok := value.(*object.Integer); ok {
            if intObj.Value != 84 {
                t.Errorf("Expected 84, got %d", intObj.Value)
            }
        } else {
            t.Error("Expected integer result")
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
dbHost, _ := p.GetVar("db_host")
cacheSize, _ := p.GetVar("cache_size")
```

### Data Processing Pipeline
```go
// Process data with Scriptling
p := scriptling.New("json")
p.SetVar("raw_data", jsonString)

pipeline := `
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