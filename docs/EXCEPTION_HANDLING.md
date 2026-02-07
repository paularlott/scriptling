# Exception Type Matching

Scriptling supports Python 3-style exception type matching with `except ExceptionType as e:` syntax.

## Features

### Multiple Except Blocks
```python
try:
    risky_operation()
except ValueError as e:
    print(f"Value error: {e}")
except TypeError as e:
    print(f"Type error: {e}")
except Exception as e:
    print(f"Other error: {e}")
```

### Exception Constructors
Built-in exception types can be raised using constructors:

```python
raise Exception("generic error")
raise ValueError("invalid value")
raise TypeError("wrong type")
raise NameError("name not defined")
```

### Exception Type Hierarchy
- `Exception` - catches all exceptions (except SystemExit)
- `ValueError` - value-related errors
- `TypeError` - type-related errors
- `NameError` - undefined variable/name errors
- Bare `except:` - catches everything (discouraged but valid Python 3)

### Python 3 Compatibility

```python
# ✓ Supported (Python 3 style)
try:
    x = 1 / 0
except Exception as e:
    print(f"Error: {e}")

# ✓ Supported (bare except, discouraged but valid)
try:
    x = 1 / 0
except:
    print("Error occurred")

# ✗ NOT supported (Python 2 only)
try:
    raise "string error"  # This will fail
except:
    pass
```

## Implementation Details

- Exception types are matched in order - first matching except block is executed
- `Exception` type catches all exceptions including specific types like `ValueError`
- Errors (like division by zero) are automatically converted to appropriate Exception types
- Exception variable binding works with all exception types
- Multiple except blocks are fully supported

## Examples

### Specific Exception Handling
```python
try:
    value = int("not a number")
except ValueError as e:
    print(f"Could not convert: {e}")
```

### Exception Hierarchy
```python
try:
    raise ValueError("test")
except Exception as e:
    # This catches ValueError since Exception is the base type
    print(f"Caught: {e}")
```

### Multiple Handlers
```python
def process_data(data):
    try:
        result = risky_operation(data)
        return result
    except ValueError as e:
        log_error(f"Invalid data: {e}")
        return None
    except TypeError as e:
        log_error(f"Wrong type: {e}")
        return None
    except Exception as e:
        log_error(f"Unexpected error: {e}")
        raise  # Re-raise unexpected errors
```

### Automatic Type Inference

Scriptling automatically infers exception types from error messages:

```python
try:
    x = "string" + 123  # Type mismatch
except TypeError as e:
    print("Caught type error")  # This works!

try:
    x = undefined_variable
except NameError as e:
    print("Caught name error")  # This works!
```
