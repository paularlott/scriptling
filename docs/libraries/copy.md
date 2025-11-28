# copy Library

The `copy` library provides shallow and deep copy operations for Python-compatible data structures.

## Import

```python
import copy
```

## Functions

### copy

#### `copy(obj)`
Create a shallow copy of an object.

A shallow copy creates a new object, but nested objects are still references to the originals.

```python
# Copy a list
original = [1, 2, 3]
copied = copy.copy(original)

# Modifying copied doesn't affect original
copied.append(4)
# original is still [1, 2, 3]
# copied is [1, 2, 3, 4]
```

**Shallow copy behavior with nested structures:**
```python
original = [[1, 2], [3, 4]]
copied = copy.copy(original)

# The outer list is new, but inner lists are shared
# Modifying the inner list affects both
```

### deepcopy

#### `deepcopy(obj)`
Create a deep copy of an object.

A deep copy recursively copies all nested objects, creating completely independent copies.

```python
# Deep copy a nested structure
original = [[1, 2], [3, 4]]
copied = copy.deepcopy(original)

# All nested objects are also copied
# Modifying copied doesn't affect original at all
```

## Supported Types

Both `copy()` and `deepcopy()` support:

| Type | Behavior |
|------|----------|
| `list` | Creates new list (shallow/deep copies elements) |
| `dict` | Creates new dict (shallow/deep copies values) |
| `tuple` | Creates new tuple (shallow/deep copies elements) |
| `int`, `float`, `str`, `bool` | Returns same value (immutable) |
| `None` | Returns None |

## Examples

### Copying a simple list
```python
import copy

original = [1, 2, 3, 4, 5]

# Both copy and deepcopy work the same for flat lists
shallow = copy.copy(original)
deep = copy.deepcopy(original)

# All three are independent
shallow.append(6)
deep.append(7)

# original: [1, 2, 3, 4, 5]
# shallow:  [1, 2, 3, 4, 5, 6]
# deep:     [1, 2, 3, 4, 5, 7]
```

### Copying a dict
```python
import copy

original = {"name": "Alice", "age": 30}
copied = copy.copy(original)

copied["name"] = "Bob"
# original["name"] is still "Alice"
```

### Deep copying nested structures
```python
import copy

# A complex nested structure
data = {
    "users": [
        {"name": "Alice", "scores": [95, 87, 92]},
        {"name": "Bob", "scores": [78, 82, 88]}
    ],
    "metadata": {"version": 1}
}

# Create a completely independent copy
backup = copy.deepcopy(data)

# Any modifications to backup don't affect data
# and vice versa
```

### When to use copy vs deepcopy

```python
import copy

# Use copy() for flat structures or when you want shared references
config_template = {"debug": False, "timeout": 30}
user_config = copy.copy(config_template)

# Use deepcopy() for nested structures when you need full independence
nested_data = [[1, 2], {"a": [3, 4]}]
safe_copy = copy.deepcopy(nested_data)
```

### Preserving immutables
```python
import copy

# Immutable types (int, float, str, bool) are returned as-is
x = 42
y = copy.copy(x)      # Same value
z = copy.deepcopy(x)  # Same value

text = "hello"
text_copy = copy.deepcopy(text)  # "hello"
```

## Differences from Python

In standard Python, `copy` also handles:
- Custom classes with `__copy__` and `__deepcopy__` methods
- Circular references (preventing infinite recursion)
- More complex types like `set`, `frozenset`, `bytearray`

The Scriptling implementation covers the most common use cases with lists, dicts, tuples, and primitive types.
