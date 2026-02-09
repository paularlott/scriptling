# Exception and Error Handling

Scriptling provides comprehensive error handling capabilities through Python 3-style exception handling.

## Overview

Scriptling supports two types of error conditions:

| Type | Description | Example |
|------|-------------|---------|
| **Errors** | Runtime errors that halt execution | Division by zero, undefined variable |
| **Exceptions** | Explicitly raised error conditions | `raise ValueError("invalid input")` |

## Basic Exception Handling

### Try/Except/Finally

The basic structure for error handling:

```python
try:
    # Code that might raise an exception
    result = 10 / 0
except ZeroDivisionError:
    # Handle division by zero
    print("Cannot divide by zero")
finally:
    # Always executes (optional)
    print("Cleanup code here")
```

### Multiple Exception Types

Handle different error types with separate except blocks:

```python
try:
    value = int(user_input)
    result = 100 / value
except ValueError as e:
    print(f"Invalid number: {e}")
except ZeroDivisionError:
    print("Cannot divide by zero")
except Exception as e:
    print(f"Unexpected error: {e}")
```

## Exception Type Hierarchy

Scriptling supports Python 3-style exception type matching:

```
Exception (base class)
├── ValueError     - Invalid values
├── TypeError      - Type mismatches
├── NameError      - Undefined names
├── ZeroDivisionError - Division by zero
└── ... (more specific types)
```

### Built-in Exception Types

| Exception Type | When Raised |
|----------------|-------------|
| `Exception` | Base class for all exceptions |
| `ValueError` | Invalid value for operation |
| `TypeError` | Operation on wrong type |
| `NameError` | Variable/identifier not found |
| `ZeroDivisionError` | Division or modulo by zero |
| `IndexError` | Sequence index out of range |
| `KeyError` | Dictionary key not found |
| `AttributeError` | Attribute not found on object |

### Python 3 Compatibility

```python
# ✓ Supported (Python 3 style)
try:
    x = 1 / 0
except Exception as e:
    print(f"Error: {e}")

# ✓ Supported (bare except, discouraged)
try:
    x = 1 / 0
except:
    print("Error occurred")

# ✗ NOT supported (Python 2 string exceptions)
try:
    raise "string error"  # This will fail
except:
    pass
```

## Raising Exceptions

### Basic Raise

```python
def validate_age(age):
    if age < 0:
        raise ValueError("Age cannot be negative")
    if age > 150:
        raise ValueError("Age seems unrealistic")
    return True
```

### Exception Constructors

Built-in exception types can be raised using constructors:

```python
raise Exception("generic error")
raise ValueError("invalid value")
raise TypeError("wrong type")
raise NameError("name not defined")
```

### Re-raising Exceptions

Re-raise an exception after handling:

```python
try:
    risky_operation()
except Exception as e:
    log_error(e)
    raise  # Re-raise the same exception
```

### Raise with Different Type

Change the exception type while preserving the traceback:

```python
try:
    parse_config(data)
except ValueError as e:
    raise TypeError(f"Configuration error: {e}")
```

## Exception Object Properties

When you catch an exception with `as e`, you can access its properties:

```python
try:
    result = 10 / 0
except Exception as e:
    print(f"Type: {type(e).__name__}")
    print(f"Message: {e}")
    # String representation shows the message
    print(str(e))  # "division by zero"
```

## Automatic Exception Type Inference

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

## Common Patterns

### Safe Dictionary Access

```python
# Option 1: Using try/except
try:
    value = data["key"]
except KeyError:
    value = default_value

# Option 2: Using get() method (preferred)
value = data.get("key", default_value)
```

### Safe List Access

```python
try:
    item = items[index]
except IndexError:
    item = None
```

### Resource Cleanup with Finally

```python
file = None
try:
    file = open_file("data.txt")
    process_file(file)
except Exception as e:
    print(f"Error: {e}")
finally:
    # Always cleanup, even if exception occurred
    if file:
        file.close()
```

### Context Managers (when available)

```python
# Prefer context managers when available
try:
    with open_file("data.txt") as file:
        process_file(file)
except Exception as e:
    print(f"Error: {e}")
```

## Error vs Exception Best Practices

### When to Use Errors (Return Error Objects)

```python
# For expected failure cases
def divide(a, b):
    if b == 0:
        return error("Division by zero")
    return a / b

result = divide(10, 0)
if is_error(result):
    print("Operation failed:", result)
```

### When to Use Exceptions (Raise)

```python
# For programming errors and unexpected conditions
def calculate_percentage(value, total):
    if total == 0:
        raise ValueError("Total cannot be zero")
    return (value / total) * 100

try:
    percent = calculate_percentage(50, 0)
except ValueError as e:
    print(f"Invalid input: {e}")
```

## Custom Exception Patterns

### Creating Custom Error Messages

```python
def validate_user(user):
    if not user.get("name"):
        raise ValueError("User must have a name")
    if not user.get("email"):
        raise ValueError("User must have an email")
    if "@" not in user["email"]:
        raise ValueError("Invalid email format")
    return True
```

### Exception Chaining

```python
def load_config(path):
    try:
        data = read_file(path)
        return parse_json(data)
    except FileNotFoundError:
        raise ValueError(f"Config file not found: {path}")
    except JSONParseError as e:
        raise ValueError(f"Invalid config format: {e}")
```

## Performance Considerations

### Try/Except vs Conditional Checks

```python
# Slower for frequent expected failures
try:
    value = dict["key"]
except KeyError:
    value = default

# Faster for expected lookups
value = dict.get("key", default)
```

**Rule of thumb**: Use exceptions for exceptional cases, not for control flow.

## Debugging with Exceptions

### Preserving Stack Traces

```python
def inner():
    raise ValueError("Inner error")

def outer():
    inner()  # Stack trace shows full call path
```

### Adding Context to Exceptions

```python
def process_user_input(user_input):
    try:
        value = int(user_input)
    except ValueError as e:
        # Add context while preserving original error
        raise ValueError(f"Failed to parse '{user_input}': {e}")
```

## Exception Handling in Libraries

When writing libraries, follow these guidelines:

1. **Document exceptions** your functions can raise
2. **Use specific exception types** for different error conditions
3. **Preserve original exceptions** when wrapping errors
4. **Consider recovery scenarios** - can the caller reasonably recover?

```python
# Good library design
def parse_date(date_string):
    """
    Parse a date string into a Date object.

    Args:
        date_string: Date in YYYY-MM-DD format

    Returns:
        Date object

    Raises:
        ValueError: If date_string is not valid format
        TypeError: If date_string is not a string
    """
    if not isinstance(date_string, str):
        raise TypeError("date_string must be a string")
    # ... parsing logic
```

## Common Pitfalls

### Catching Too Broadly

```python
# Bad - catches everything including system exits
try:
    some_operation()
except Exception:
    pass  # Silently ignores all errors

# Good - catch specific exceptions
try:
    some_operation()
except (ValueError, TypeError) as e:
    log_error(e)
    # Handle specific expected errors
```

### Silent Failures

```python
# Bad - silently ignores errors
try:
    risky_operation()
except:
    pass  # What went wrong?

# Good - at least log the error
try:
    risky_operation()
except Exception as e:
    logger.error(f"Operation failed: {e}")
```

### Overly Broad Try Blocks

```python
# Bad - too much code in try block
try:
    config = load_config()
    connect_database()
    process_data()
    save_results()
except Exception:
    handle_error()  # Which part failed?

# Good - narrow try blocks
config = load_config()
connect_database()
try:
    process_data()  # Just the risky part
except DataError as e:
    handle_data_error(e)
save_results()
```

## Summary

- Use `try/except/finally` for structured error handling
- Catch specific exception types when possible
- Use exceptions for exceptional cases, not control flow
- Always preserve original exceptions when wrapping errors
- Document which exceptions your functions can raise
- Add context to exceptions to aid debugging
- Avoid silent failures and overly broad exception handlers
