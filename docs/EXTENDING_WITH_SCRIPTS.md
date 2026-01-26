# Extending Scriptling with Scripts

In addition to registering Go functions and libraries, you can also register functions and libraries written in Scriptling itself. This is useful for:
- Creating reusable Scriptling code that can be shared across multiple scripts
- Building higher-level abstractions on top of Go functions
- Organizing complex Scriptling logic into modular libraries

## RegisterScriptFunc

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

### With Default Parameters

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

### Lambda Functions

```go
err := p.RegisterScriptFunc("double", `lambda x: x * 2`)

p.Eval(`
result = double(21)
print(result)  # 42
`)
```

### With Variadic Arguments

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

## RegisterScriptLibrary

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

## Adding Classes

You can define classes directly in your Scriptling libraries. These classes can then be imported and used in other scripts.

### Registering a Library with Classes

```go
err := p.RegisterScriptLibrary("shapes", `
class Rectangle:
    def __init__(self, width, height):
        self.width = width
        self.height = height

    def area(self):
        return self.width * self.height

    def perimeter(self):
        return 2 * (self.width + self.height)

class Circle:
    def __init__(self, radius):
        self.radius = radius

    def area(self):
        return 3.14159 * self.radius * self.radius
`)

p.Eval(`
import shapes

rect = shapes.Rectangle(10, 20)
print("Rectangle Area: " + str(rect.area()))
print("Rectangle Perimeter: " + str(rect.perimeter()))

circ = shapes.Circle(5)
print("Circle Area: " + str(circ.area()))
`)
```

## Documenting Scriptling Libraries

Scriptling libraries support documentation through docstrings, similar to Python:

### Module Documentation

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

### Function Documentation

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

### Help System Integration

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

## Nested Imports

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

## Using Standard Libraries in Scriptling Libraries

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

## Combining Go and Scriptling Libraries

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
    p.RegisterLibrary(object.NewLibrary("inline", map[string]*object.Builtin{
        "sqrt": {
            Fn: func(ctx context.Context, kwargs object.Kwargs, args ...object.Object) object.Object {
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

## Best Practices for Scriptling Libraries

1. **Keep Libraries Focused**: Each library should have a clear, single purpose
2. **Use Descriptive Names**: Function and constant names should be self-explanatory
3. **Document Complex Logic**: Add comments in your Scriptling code
4. **Handle Errors Gracefully**: Return meaningful error messages
5. **Avoid Side Effects**: Libraries should be pure when possible
6. **Test Thoroughly**: Test your libraries with various inputs

## Example: Complete Scriptling Library

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

