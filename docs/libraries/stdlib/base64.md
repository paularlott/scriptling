# Base64 Library

Functions for Base64 encoding and decoding. Python-compatible function names.

## Available Functions

| Function       | Description               |
| -------------- | ------------------------- |
| `b64encode(s)` | Encode a string to Base64 |
| `b64decode(s)` | Decode a Base64 string    |

## Functions

### base64.b64encode(s)

Encodes a string to Base64.

**Parameters:**

- `s`: String to encode

**Returns:** String (Base64 encoded)

**Example:**

```python
import base64

encoded = base64.b64encode("hello world")
print(encoded)  # "aGVsbG8gd29ybGQ="
```

### base64.b64decode(s)

Decodes a Base64 string.

**Parameters:**

- `s`: Base64 string to decode

**Returns:** String (decoded)

**Example:**

```python
import base64

decoded = base64.b64decode("aGVsbG8gd29ybGQ=")
print(decoded)  # "hello world"
```

## Usage Example

```python
import base64

# Encode
original = "Hello, World!"
encoded = base64.b64encode(original)
print("Encoded:", encoded)

# Decode
decoded = base64.b64decode(encoded)
print("Decoded:", decoded)

# Verify
print("Match:", original == decoded)  # True
```
