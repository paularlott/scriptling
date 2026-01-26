# Adding Help Documentation

Scriptling includes a built-in `help()` function that provides Python-like help for functions and libraries. To make your custom functions and libraries discoverable and well-documented, you should add help text.

## Using the Help System

The `help()` function works like Python's help system:

```python
# Show general help
help()

# List all available libraries
help("modules")

# List all builtin functions
help("builtins")

# Get help for a specific builtin function
help("print")
help("len")

# Get help for a library
import math
help("math")

# Get help for a library function
help("math.sqrt")

# Get help for user-defined functions
def my_function(a, b=10):
    return a + b

help("my_function")
help(my_function)  # Can also pass the function object
```

## Adding Help to User Functions (Docstrings)

Scriptling supports Python-style docstrings. If the first statement in your function body is a string literal, it will be used as the function's documentation.

```python
def calculate_area(radius):
    """Calculate the area of a circle.

    Parameters:
      radius - The radius of the circle

    Returns:
      The area as a float
    """
    return 3.14159 * radius * radius

help(calculate_area)
```

## Adding Help to Go Functions

When registering Go functions, you can provide documentation by passing help text as an optional parameter to `RegisterFunc`:

```go
p := scriptling.New()

p.RegisterFunc("my_func", func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
    // Implementation
    return object.NULL
}, `my_func() - Description of the function

Detailed documentation here.`)
```

If you omit the help text or pass an empty string, basic help will be auto-generated:

```go
p.RegisterFunc("my_func", func(ctx context.Context, args ...object.Object) object.Object {
    // Implementation
    return object.NULL
})  // Auto-generates: "my_func(...) - User-defined function"
```

## Adding Help to Builtin Functions

When creating builtin functions, add a `HelpText` field to the `Builtin` struct:

```go
package evaluator

var builtins = map[string]*object.Builtin{
    "my_func": {
        Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
            // Function implementation
            return &object.String{Value: "result"}
        },
        HelpText: `my_func(arg1, arg2) - Brief description

  Detailed description of what the function does.

  Parameters:
    arg1 - Description of first parameter
    arg2 - Description of second parameter

  Returns:
    Description of return value
        `,
    },
}
```

## Adding Help to Scriptling Libraries

Scriptling libraries support documentation through docstrings:

### Module Documentation

Add a module docstring at the top of the library script:

```go
# Register a documented library
err := p.RegisterScriptLibrary("mylib", `
"""My Library

This library provides useful utilities for common tasks.
It includes functions for data processing and formatting.
"""

def process_data(data):
    """Process input data.

    Args:
        data: The data to process

    Returns:
        Processed data
    """
    return data.upper()

def format_output(value):
    """Format a value for display.

    Args:
        value: Value to format

    Returns:
        Formatted string
    """
    return str(value)
`)
```

### Function Documentation

Document functions using docstrings (first statement in function body):

```python
def my_function(param1, param2):
    """Brief description of what the function does.

    More detailed description if needed.

    Args:
        param1: Description of first parameter
        param2: Description of second parameter

    Returns:
        Description of return value
    """
    return param1 + param2
```

### Accessing Help

```python
import mylib

help(mylib)           # Shows module docstring and functions
help(mylib.my_function)  # Shows function docstring
```

## Adding Help to Go Libraries

Go libraries can include a description that will be displayed when users call `help()` on the library:

```go
package mylib

import (
    "context"
    "github.com/paularlott/scriptling/object"
)

var MyLibrary = object.NewLibrary("mylib", map[string]*object.Builtin{
    "process": {
        Fn: func(ctx context.Context, args ...object.Object) object.Object {
            // Implementation
            return &object.String{Value: "processed"}
        },
        HelpText: `process(data) - Process the input data`,
    },
}, nil, "My custom data processing library")
```

The description will be shown when users call `help("mylib")`.

## Adding Help to Library Functions

When creating libraries, add `HelpText` to each function in the library:

```go
package mylib

import (
    "context"
    "github.com/paularlott/scriptling/object"
)

var MyLibrary = object.NewLibrary("mylib", map[string]*object.Builtin{
    "process": {
        Fn: func(ctx context.Context, args ...object.Object) object.Object {
            // Implementation
            return &object.String{Value: "processed"}
        },
        HelpText: `process(data) - Process the input data

  Takes input data and processes it according to the library's rules.

  Parameters:
    data - The data to process (string or list)

  Returns:
    Processed data as a string

  Examples:
    mylib.process("hello")
    mylib.process([1, 2, 3])`,
    },
    "validate": {
        Fn: func(ctx context.Context, args ...object.Object) object.Object {
            // Implementation
            return &object.Boolean{Value: true}
        },
        HelpText: `validate(input) - Validate input data

  Checks if the input meets the required criteria.

  Parameters:
    input - The data to validate

  Returns:
    True if valid, False otherwise

  Examples:
    mylib.validate("test@example.com")`,
    },
}, nil, "My custom data processing library")
```

## Adding Help Using the Builder Pattern

Scriptling also provides a builder pattern for creating functions, libraries, and classes with help text.

### Function Builder

The `FunctionBuilder` allows you to create individual typed Go functions with automatic type conversion.

```go
import (
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/object"
)

p := scriptling.New()

// Build a function with help text
fb := object.NewFunctionBuilder()
fn := fb.FunctionWithHelp(func(a, b int) int {
    return a + b
}, `add(a, b) - Add two integers

  Parameters:
    a - First integer
    b - Second integer

  Returns:
    The sum of a and b`).
    Build()

p.RegisterFunc("add", fn)
```

You can also register a function without help text:

```go
fb := object.NewFunctionBuilder()
fn := fb.Function(func(a, b int) int {
    return a + b
}).Build()
p.RegisterFunc("add", fn)
```

### Library Builder

The `LibraryBuilder` creates complete libraries with multiple functions, constants, and a description.

```go
// Build a library with description and functions
library := object.NewLibraryBuilder("mylib", "My custom data processing library").
    FunctionWithHelp("process", func(data string) string {
        return "processed: " + data
    }, `process(data) - Process the input data

  Takes input data and processes it.

  Parameters:
    data - The data to process

  Returns:
    Processed data as string`).
    FunctionWithHelp("validate", func(input string) bool {
        return len(input) > 0
    }, `validate(input) - Validate input data

  Checks if the input meets criteria.

  Parameters:
    input - The data to validate

  Returns:
    True if valid, False otherwise`).
    Constant("MAX_SIZE", 1000).
    Constant("VERSION", "1.0.0").
    Build()

p.RegisterLibrary( library)
```

### Class Builder

The `ClassBuilder` creates classes with typed methods.

```go
// Build a class with methods
classBuilder := object.NewClassBuilder("MyClass")
classBuilder.Method("__init__", func(self *object.Instance, name string) {
    self.Fields["name"] = &object.String{Value: name}
})
classBuilder.Method("get_data", func(self *object.Instance) string {
    name, _ := self.Fields["name"].AsString()
    return "data for " + name
})
classBuilder.Method("set_data", func(self *object.Instance, data string) {
    self.Fields["data"] = &object.String{Value: data}
})
myClass := classBuilder.Build()

// Register through a library
p.RegisterLibrary(object.NewLibrary("inline", nil, map[string]object.Object{
    "MyClass": myClass,
}, "My library"))
```

### Registering Script Functions with Help

When registering functions from scripts, docstrings in the script become the help text:

```go
// Script with docstrings - first string literal in function becomes help
script := `
def process_data(data):
    """Process input data and return result.

    Args:
        data: The data to process

    Returns:
        Processed data
    """
    return data.upper()

def format_output(value):
    """Format a value for display.

    Args:
        value: Value to format

    Returns:
        Formatted string
    """
    return str(value)
`

// Register the script - docstrings are automatically extracted
err := p.RegisterScriptFunc("process_data", script)
err = p.RegisterScriptFunc("format_value", script)
```

### Registering Script Libraries with Help

```go
// Register a complete library with module docstring and function docstrings
err := p.RegisterScriptLibrary("mylib", `
"""My Library - Custom data processing utilities.

This library provides functions for data processing and formatting.
"""

def process(data):
    """Process input data.

    Args:
        data: Input string or list

    Returns:
        Processed data
    """
    if isinstance(data, str):
        return data.upper()
    return data

def format(value, fmt_type="default"):
    """Format a value for display.

    Args:
        value: Value to format
        fmt_type: Format type (default: "default")

    Returns:
        Formatted string
    """
    return str(value)
`)

// Users can now access help
// help("mylib")       # Shows module docstring
// help("mylib.process")  # Shows function docstring
```

## Manual Registration Summary

| Method | Use Case | Help Source |
|--------|-----------|-------------|
| `RegisterFunc(name, func, help)` | Single Go function | Help text parameter |
| `RegisterScriptFunc(name, script)` | Function from script | Docstring in script |
| `RegisterLibrary(library)` | Pre-built Go library | Library's HelpText fields |
| `RegisterScriptLibrary(name, script)` | Library from script | Module/function docstrings |
| `NewFunctionBuilder().FunctionWithHelp(fn, help)` | Builder pattern | Help text parameter |
| `NewLibraryBuilder(name, description)` | Builder pattern | Description + FunctionWithHelp |
| `NewClassBuilder(name).Method(...)` | Builder pattern | Methods automatically added |

## Help Text Best Practices

1. **First Line**: Start with the function signature and a brief one-line description
2. **Blank Line**: Add a blank line after the first line
3. **Detailed Description**: Provide a more detailed explanation if needed
4. **Parameters Section**: List each parameter with its description
5. **Returns Section**: Describe what the function returns
6. **Examples Section**: Provide practical examples of usage
7. **Formatting**: Use consistent indentation (2 spaces recommended)

## Notes

- **Optional**: The `HelpText` field is optional. If not provided, `help()` will show "No documentation available"
- **User Functions**: User-defined functions automatically show their parameter information, including default values and variadic parameters
- **Libraries**: When you call `help("library_name")`, it lists all functions in the library
- **Discoverability**: Use `help("modules")` to see all imported and available libraries
- **Consistency**: Follow Python's help text conventions for familiarity

By adding comprehensive help text to your functions and libraries, you make them much easier to use and discover for Scriptling users.
