# Extending Scriptling - Libraries and Functions Guide

This guide covers how to create custom libraries and functions for Scriptling in Go, including advanced features like output capture.

## Function Signature

All Scriptling functions in Go must follow this signature:

```go
func(ctx context.Context, args ...object.Object) object.Object
```

- `ctx`: Context containing environment and other runtime information
- `args`: Variable number of Scriptling objects passed from the script
- Returns: A Scriptling object result

## Basic Function Creation

### Simple Function

```go
package main

import (
    "context"
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
)

func main() {
    p := scriptling.New()

    // Register a simple function
    p.RegisterFunc("double", func(ctx context.Context, args ...object.Object) object.Object {
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

### Function with Output Capture

```go
import (
    "context"
    "fmt"
    "github.com/paularlott/scriptling/evaluator"
)

p.RegisterFunc("debug_print", func(ctx context.Context, args ...object.Object) object.Object {
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

## Advanced Function Examples

### File Operations with Output Capture

```go
p.RegisterFunc("read_file", func(ctx context.Context, args ...object.Object) object.Object {
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
```

### HTTP Client with Logging

```go
import (
    "net/http"
    "io"
    "time"
)

p.RegisterFunc("http_get", func(ctx context.Context, args ...object.Object) object.Object {
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
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
                elements := make([]object.Object, len(l.messages))
                for i, msg := range l.messages {
                    elements[i] = &object.String{Value: msg}
                }
                return &object.List{Elements: elements}
            },
        },
        "clear": {
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
p.RegisterFunc("safe_divide", func(ctx context.Context, args ...object.Object) object.Object {
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

### 2. Type Conversion Helpers

```go
// Helper function for type conversion
func toFloat(obj object.Object) (float64, bool) {
    switch o := obj.(type) {
    case *object.Float:
        return o.Value, true
    case *object.Integer:
        return float64(o.Value), true
    default:
        return 0, false
    }
}

func toString(obj object.Object) (string, bool) {
    if strObj, ok := obj.(*object.String); ok {
        return strObj.Value, true
    }
    return "", false
}

// Usage
p.RegisterFunc("power", func(ctx context.Context, args ...object.Object) object.Object {
    if len(args) != 2 {
        return &object.String{Value: "Error: power requires 2 arguments"}
    }

    base, ok := toFloat(args[0])
    if !ok {
        return &object.String{Value: "Error: base must be number"}
    }

    exp, ok := toFloat(args[1])
    if !ok {
        return &object.String{Value: "Error: exponent must be number"}
    }

    result := math.Pow(base, exp)
    return &object.Float{Value: result}
})
```

### 3. Context Usage

```go
p.RegisterFunc("long_operation", func(ctx context.Context, args ...object.Object) object.Object {
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
    p.RegisterFunc("test_func", func(ctx context.Context, args ...object.Object) object.Object {
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
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
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
            Fn: func(ctx context.Context, args ...object.Object) object.Object {
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

## Registering Scriptling Functions and Libraries

In addition to registering Go functions and libraries, you can also register functions and libraries written in Scriptling itself. This is useful for:
- Creating reusable Scriptling code that can be shared across multiple scripts
- Building higher-level abstractions on top of Go functions
- Organizing complex Scriptling logic into modular libraries

### RegisterScriptFunc

Register a function written in Scriptling:

```go
package main

import (
    "fmt"
    "github.com/paularlott/scriptling"
)

func main() {
    p := scriptling.New()

    // Register a Scriptling function
    err := p.RegisterScriptFunc("calculate_area", `
def calculate_area(width, height):
    return width * height
calculate_area
`)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    // Use the registered function
    p.Eval(`
area = calculate_area(10, 20)
print("Area: " + str(area))  # Area: 200
`)
}
```

The script must evaluate to a function (either a `def` or `lambda`). The last line should be the function name to return it.

#### With Default Parameters

```go
err := p.RegisterScriptFunc("format_name", `
def format_name(first, last, title="Mr."):
    return title + " " + first + " " + last
format_name
`)

p.Eval(`
name1 = format_name("John", "Doe")
name2 = format_name("Jane", "Smith", "Dr.")
print(name1)  # Mr. John Doe
print(name2)  # Dr. Jane Smith
`)
```

#### Lambda Functions

```go
err := p.RegisterScriptFunc("double", `lambda x: x * 2`)

p.Eval(`
result = double(21)
print(result)  # 42
`)
```

#### With Variadic Arguments

```go
err := p.RegisterScriptFunc("sum_all", `
def sum_all(*args):
    total = 0
    for x in args:
        total = total + x
    return total
sum_all
`)

p.Eval(`
result = sum_all(1, 2, 3, 4, 5)
print(result)  # 15
`)
```

### RegisterScriptLibrary

Register a library written in Scriptling:

```go
package main

import (
    "fmt"
    "github.com/paularlott/scriptling"
)

func main() {
    p := scriptling.New()

    // Register a Scriptling library
    err := p.RegisterScriptLibrary("mathutils", `
def square(x):
    return x * x

def cube(x):
    return x * x * x

def sum_of_squares(a, b):
    return square(a) + square(b)

PI = 3.14159
E = 2.71828
`)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    // Use the library
    p.Eval(`
import mathutils

print("Square of 5: " + str(mathutils.square(5)))
print("Cube of 3: " + str(mathutils.cube(3)))
print("Sum of squares: " + str(mathutils.sum_of_squares(3, 4)))
print("PI: " + str(mathutils.PI))
`)
}
```

### Documenting Scriptling Libraries

Scriptling libraries support documentation through docstrings, similar to Python:

#### Module Documentation

Add a module docstring at the top of the library script (first statement must be a string literal):

```go
err := p.RegisterScriptLibrary("mathutils", `
"""Math Utilities Library

This library provides basic mathematical operations and constants.
It includes functions for arithmetic and common mathematical constants.
"""

def square(x):
    """Return the square of x."""
    return x * x

def cube(x):
    """Return the cube of x."""
    return x * x * x

PI = 3.14159
E = 2.71828
`)
```

The module docstring will be displayed when using `help("library_name")`.

#### Function Documentation

Document individual functions using docstrings (first statement in function body):

```go
def add(a, b):
    """Add two numbers together.

    Args:
        a: First number
        b: Second number

    Returns:
        The sum of a and b
    """
    return a + b
```

Function docstrings are displayed when using `help("library.function_name")`.

#### Help System Integration

Once documented, users can access help:

```python
import mathutils

help(mathutils)        # Shows module docstring and available functions
help(mathutils.add)    # Shows function docstring
```

The library script can define:
- Functions (using `def`)
- Lambda functions
- Constants and variables
- Any other Scriptling code

All defined names (except `import`) will be available when the library is imported.

**Note**: Script libraries are lazily loaded. The script is only evaluated the first time it is imported. This means syntax errors or runtime errors in the library script will only be reported when the library is actually used.

### Nested Imports

Scriptling libraries can import other libraries, including:
- Other Scriptling libraries
- Go libraries
- Standard libraries

```go
package main

import (
    "fmt"
    "github.com/paularlott/scriptling"
)

func main() {
    p := scriptling.New()

    // Register a base library
    err := p.RegisterScriptLibrary("geometry_base", `
def distance(x1, y1, x2, y2):
    dx = x2 - x1
    dy = y2 - y1
    return (dx * dx + dy * dy) ** 0.5
`)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    // Register a library that imports the base library
    err = p.RegisterScriptLibrary("geometry_advanced", `
import geometry_base

def circle_circumference(radius):
    return 2 * 3.14159 * radius

def distance_from_origin(x, y):
    return geometry_base.distance(0, 0, x, y)
`)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    // Use the advanced library
    p.Eval(`
import geometry_advanced

circ = geometry_advanced.circle_circumference(5)
dist = geometry_advanced.distance_from_origin(3, 4)

print("Circumference: " + str(circ))  # 31.4159
print("Distance: " + str(dist))       # 5.0
`)
}
```

### Using Standard Libraries in Scriptling Libraries

```go
// Register a library that uses the json standard library
err := p.RegisterScriptLibrary("data_processor", `
import json

def parse_user(json_str):
    user = json.loads(json_str)
    return user["name"] + " (" + str(user["age"]) + ")"

def create_user_json(name, age):
    data = {"name": name, "age": age}
    return json.dumps(data)
`)

p.Eval(`
import data_processor

user_json = data_processor.create_user_json("Alice", 30)
print("JSON: " + user_json)

parsed = data_processor.parse_user(user_json)
print("Parsed: " + parsed)  # Alice (30)
`)
```

### Combining Go and Scriptling Libraries

You can create Scriptling libraries that build on top of Go libraries:

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

    // Register a Go library with description
    p.RegisterLibrary("gomath", object.NewLibrary(map[string]*object.Builtin{
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
        },
    }, nil, "Custom mathematical functions library"))

    // Register a Scriptling library that uses the Go library
    err := p.RegisterScriptLibrary("advanced_math", `
import gomath

def pythagorean(a, b):
    c_squared = a * a + b * b
    return gomath.sqrt(c_squared)

def distance_3d(x1, y1, z1, x2, y2, z2):
    dx = x2 - x1
    dy = y2 - y1
    dz = z2 - z1
    return gomath.sqrt(dx*dx + dy*dy + dz*dz)
`)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    p.Eval(`
import advanced_math

hypotenuse = advanced_math.pythagorean(3.0, 4.0)
print("Hypotenuse: " + str(hypotenuse))  # 5.0

dist = advanced_math.distance_3d(0.0, 0.0, 0.0, 1.0, 2.0, 2.0)
print("3D Distance: " + str(dist))  # 3.0
`)
}
```

### Best Practices for Scriptling Libraries

1. **Keep Libraries Focused**: Each library should have a clear, single purpose
2. **Use Descriptive Names**: Function and constant names should be self-explanatory
3. **Document Complex Logic**: Add comments in your Scriptling code
4. **Handle Errors Gracefully**: Return meaningful error messages
5. **Avoid Side Effects**: Libraries should be pure when possible
6. **Test Thoroughly**: Test your libraries with various inputs

### Example: Complete Scriptling Library

```go
err := p.RegisterScriptLibrary("string_utils", `
def capitalize_words(text):
    """Capitalize the first letter of each word"""
    words = text.split(" ")
    result = []
    for word in words:
        if len(word) > 0:
            capitalized = word[0].upper() + word[1:].lower()
            append(result, capitalized)
    return join(result, " ")

def reverse_string(text):
    """Reverse a string"""
    chars = []
    for i in range(len(text) - 1, -1, -1):
        append(chars, text[i])
    return join(chars, "")

def count_vowels(text):
    """Count the number of vowels in a string"""
    vowels = "aeiouAEIOU"
    count = 0
    for char in text:
        if char in vowels:
            count = count + 1
    return count

# Constants
VOWELS = "aeiouAEIOU"
CONSONANTS = "bcdfghjklmnpqrstvwxyzBCDFGHJKLMNPQRSTVWXYZ"
`)

p.Eval(`
import string_utils

text = "hello world"
print(string_utils.capitalize_words(text))  # Hello World
print(string_utils.reverse_string(text))    # dlrow olleh
print(string_utils.count_vowels(text))      # 3
`)
```

This guide provides comprehensive examples of how to extend Scriptling with custom functions and libraries, including proper use of output capture and context handling.