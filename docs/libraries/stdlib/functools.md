# functools Library

The `functools` library provides higher-order functions that act on or return other functions, compatible with Python's `functools` module.

## Import

```python
import functools
```

## Functions

### reduce(function, iterable[, initializer])

Apply a function of two arguments cumulatively to the items of an iterable, from left to right, to reduce the iterable to a single value.

**Parameters:**
- `function` - A function taking two arguments (accumulator, current_item)
- `iterable` - A list of items to process
- `initializer` - Optional. Starting value for the accumulator

**Returns:** The final accumulated value

**Examples:**

```python
import functools

# Sum a list
def add(x, y):
    return x + y

result = functools.reduce(add, [1, 2, 3, 4, 5])  # Returns 15
```

```python
# With initial value
result = functools.reduce(add, [1, 2, 3], 10)  # Returns 16
```

```python
# Product of a list
def multiply(x, y):
    return x * y

result = functools.reduce(multiply, [1, 2, 3, 4])  # Returns 24
```

```python
# Find maximum
def max_val(a, b):
    if a > b:
        return a
    return b

result = functools.reduce(max_val, [3, 1, 4, 1, 5, 9])  # Returns 9
```

```python
# Concatenate strings
def concat(a, b):
    return a + b

result = functools.reduce(concat, ["a", "b", "c"])  # Returns "abc"
```

## How reduce() Works

`reduce()` applies a function cumulatively:

```
reduce(add, [1, 2, 3, 4, 5])
```

Is equivalent to:
```
add(add(add(add(1, 2), 3), 4), 5)
= add(add(add(3, 3), 4), 5)
= add(add(6, 4), 5)
= add(10, 5)
= 15
```

With an initializer:
```
reduce(add, [1, 2, 3], 10)
```

Is equivalent to:
```
add(add(add(10, 1), 2), 3)
= add(add(11, 2), 3)
= add(13, 3)
= 16
```

## Use Cases

### Summing Values
```python
import functools

def add(x, y):
    return x + y

total = functools.reduce(add, [100, 200, 300])  # 600
```

### Building Data Structures
```python
import functools

def merge_dicts(acc, item):
    acc[item[0]] = item[1]
    return acc

pairs = [["a", 1], ["b", 2], ["c", 3]]
result = functools.reduce(merge_dicts, pairs, {})
# {"a": 1, "b": 2, "c": 3}
```

### Processing Pipelines
```python
import functools

def apply_fn(value, fn):
    return fn(value)

def double(x):
    return x * 2

def add_one(x):
    return x + 1

functions = [double, add_one, double]
result = functools.reduce(apply_fn, functions, 5)
# double(5) = 10, add_one(10) = 11, double(11) = 22
```

## Notes

- If the iterable is empty and no initializer is provided, an error is raised
- If the iterable has one item and no initializer, that item is returned
- The function must take exactly 2 arguments

## Python Compatibility

This library implements a subset of Python's `functools` module:

| Function | Supported |
|----------|-----------|
| reduce | ✅ |
| partial | ❌ |
| partialmethod | ❌ |
| lru_cache | ❌ |
| cache | ❌ |
| cached_property | ❌ |
| wraps | ❌ |
| total_ordering | ❌ |
| cmp_to_key | ❌ |
