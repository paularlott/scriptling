# Base64 Library

Functions for Base64 encoding and decoding.

## Functions

### base64.encode(string)

Encodes a string to Base64.

**Parameters:**
- `string`: String to encode

**Returns:** String (Base64 encoded)

**Example:**
```python
import base64

encoded = base64.encode("hello world")
print(encoded)  # "aGVsbG8gd29ybGQ="
```

### base64.decode(string)

Decodes a Base64 string.

**Parameters:**
- `string`: Base64 string to decode

**Returns:** String (decoded)

**Example:**
```python
import base64

decoded = base64.decode("aGVsbG8gd29ybGQ=")
print(decoded)  # "hello world"
```

## Usage Example

```python
import base64

# Encode
original = "Hello, World!"
encoded = base64.encode(original)
print("Encoded:", encoded)

# Decode
decoded = base64.decode(encoded)
print("Decoded:", decoded)

# Verify
print("Match:", original == decoded)  # True
```