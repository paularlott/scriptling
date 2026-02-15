# string Library

String constants for character classification, matching Python's `string` module.

## Import

```python
import string
```

## Available Constants

| Constant          | Description                        |
| ----------------- | ---------------------------------- |
| `ascii_letters`   | All ASCII letters (a-z, A-Z)       |
| `ascii_lowercase` | Lowercase ASCII letters (a-z)      |
| `ascii_uppercase` | Uppercase ASCII letters (A-Z)      |
| `digits`          | Decimal digits (0-9)               |
| `hexdigits`       | Hexadecimal digits (0-9, a-f, A-F) |
| `octdigits`       | Octal digits (0-7)                 |
| `punctuation`     | ASCII punctuation characters       |
| `whitespace`      | Whitespace characters              |

## Constants

### `ascii_letters`

Concatenation of `ascii_lowercase` and `ascii_uppercase`.

```python
string.ascii_letters  # "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
```

### `ascii_lowercase`

Lowercase ASCII letters.

```python
string.ascii_lowercase  # "abcdefghijklmnopqrstuvwxyz"
```

### `ascii_uppercase`

Uppercase ASCII letters.

```python
string.ascii_uppercase  # "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
```

### `digits`

Decimal digits.

```python
string.digits  # "0123456789"
```

### `hexdigits`

Hexadecimal digits.

```python
string.hexdigits  # "0123456789abcdefABCDEF"
```

### `octdigits`

Octal digits.

```python
string.octdigits  # "01234567"
```

### `punctuation`

ASCII punctuation characters.

```python
string.punctuation  # "!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~"
```

### `whitespace`

Whitespace characters.

```python
string.whitespace  # " \t\n\r\v\f"
```

### `printable`

Combination of digits, letters, punctuation, and whitespace.

```python
string.printable
```

## Examples

### Character Validation

```python
import string

def is_valid_identifier(s):
    if len(s) == 0:
        return False
    # First char must be letter or underscore
    if s[0] not in string.ascii_letters + "_":
        return False
    # Rest can include digits
    valid = string.ascii_letters + string.digits + "_"
    for c in s:
        if c not in valid:
            return False
    return True

print(is_valid_identifier("my_var"))   # True
print(is_valid_identifier("123abc"))   # False
```

### Generate Random String

```python
import string
import random

def random_string(length):
    chars = string.ascii_letters + string.digits
    result = ""
    for i in range(length):
        result = result + random.choice(chars)
    return result

print(random_string(10))  # e.g., "aB3xY7mK2p"
```

### Check for Hex String

```python
import string

def is_hex(s):
    for c in s:
        if c not in string.hexdigits:
            return False
    return True

print(is_hex("deadbeef"))  # True
print(is_hex("xyz123"))    # False
```

## Python Compatibility

This module provides the same constants as Python's `string` module.
