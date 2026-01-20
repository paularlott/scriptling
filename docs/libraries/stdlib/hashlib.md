# Hashlib Library

Cryptographic hash functions.

## Functions

### hashlib.md5(string)

Computes MD5 hash of a string.

**Parameters:**
- `string`: String to hash

**Returns:** String (hexadecimal hash)

**Example:**
```python
import hashlib

hash = hashlib.md5("hello")
print(hash)  # "5d41402abc4b2a76b9719d911017c592"
```

### hashlib.sha1(string)

Computes SHA-1 hash of a string.

**Parameters:**
- `string`: String to hash

**Returns:** String (hexadecimal hash)

**Example:**
```python
import hashlib

hash = hashlib.sha1("hello")
print(hash)  # "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d"
```

### hashlib.sha256(string)

Computes SHA-256 hash of a string.

**Parameters:**
- `string`: String to hash

**Returns:** String (hexadecimal hash)

**Example:**
```python
import hashlib

hash = hashlib.sha256("hello")
print(hash)  # "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
```

## Usage Example

```python
import hashlib

text = "Hello, World!"

# Different hash algorithms
md5_hash = hashlib.md5(text)
sha1_hash = hashlib.sha1(text)
sha256_hash = hashlib.sha256(text)

print("MD5:", md5_hash)
print("SHA-1:", sha1_hash)
print("SHA-256:", sha256_hash)

# Hash consistency
hash1 = hashlib.sha256("test")
hash2 = hashlib.sha256("test")
print("Consistent:", hash1 == hash2)  # True
```