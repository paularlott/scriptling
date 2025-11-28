# Scriptling Extension Examples

This directory contains a comprehensive example of how to extend Scriptling with custom functions and libraries written in Go.

## main.go

A complete example demonstrating:

- **Custom Functions**: Registering individual functions with `RegisterFunc()`
- **Custom Libraries**: Creating and registering libraries with `RegisterLibrary()`
- **Argument Handling**: Accepting and validating function arguments
- **Return Values**: Returning different types of results
- **Type Casting**: Converting between Scriptling object types (Integer, Float, String, List, Dict)
- **Arrays**: Processing Scriptling lists/arrays
- **Maps**: Working with Scriptling dictionaries/maps

### Features Demonstrated

1. **Simple Function**: `greet(name)` - Takes a string, returns a greeting
2. **Array Processing**: `process_numbers(numbers)` - Takes an array of numbers, returns processed strings
3. **Library Functions**:
   - `mathutils.power(base, exp)` - Calculates power with type casting
   - `mathutils.sum_array(numbers)` - Sums all numbers in an array
   - `mathutils.get_map_value(map, key)` - Retrieves values from dictionaries
   - `mathutils.create_person(name, age)` - Creates and returns a person dictionary

### Running the Example

```bash
cd examples/extending
go run .
```

This will execute a comprehensive Scriptling script that demonstrates all the extension features.

## Key Concepts

### Function Signature
All Scriptling functions must follow this signature:
```go
func(ctx context.Context, kwargs map[string]object.Object, args ...object.Object) object.Object
```

### Type System
Scriptling objects include:
- `*object.String` - String values
- `*object.Integer` - Integer values
- `*object.Float` - Float values
- `*object.Boolean` - Boolean values
- `*object.List` - Lists/arrays
- `*object.Dict` - Dictionaries

### Error Handling
Return error messages as strings:
```go
return &object.String{Value: "Error: invalid argument"}
```

### Registration
- Functions: `p.RegisterFunc(name, function)`
- Libraries: `p.RegisterLibrary(name, library)`

## Advanced Topics

For more advanced examples including:
- Output capture for logging
- State management in libraries
- Database integration
- HTTP clients
- Testing custom extensions

See the main [EXTENDING_SCRIPTLING.md](../docs/EXTENDING_SCRIPTLING.md) guide.