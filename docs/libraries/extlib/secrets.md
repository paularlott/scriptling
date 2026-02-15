# secrets Library (Extended)

The `secrets` library provides functions for generating cryptographically strong random numbers suitable for managing secrets such as account authentication, tokens, and similar. This is an **extended library** that must be explicitly registered.

> **Note:** This library is enabled by default in the Scriptling CLI but must be manually registered when using the Go API.

## Import

```python
import secrets
```

## Available Functions

| Function                  | Description                       |
| ------------------------- | --------------------------------- |
| `token_bytes([nbytes])`   | Generate random byte sequence     |
| `token_hex([nbytes])`     | Generate random hex string        |
| `token_urlsafe([nbytes])` | Generate random URL-safe string   |
| `randbelow(n)`            | Random integer in range [0, n)    |
| `randbits(k)`             | Random integer with k random bits |
| `choice(sequence)`        | Random element from sequence      |

## Functions

### token_bytes([nbytes])

Generate a random byte sequence.

**Parameters:**

- `nbytes` - Number of bytes to generate (default: 32)

**Returns:** List of integers (0-255) representing the bytes

```python
import secrets
bytes = secrets.token_bytes(16)
# [142, 87, 203, 45, ...]
```

### token_hex([nbytes])

Generate a random text string in hexadecimal.

**Parameters:**

- `nbytes` - Number of random bytes (output string will be 2x this length) (default: 32)

**Returns:** Hexadecimal string

```python
import secrets
token = secrets.token_hex(16)
# "8e57cb2d3f1a9b4c..."  (32 characters)
```

### token_urlsafe([nbytes])

Generate a random URL-safe text string.

**Parameters:**

- `nbytes` - Number of random bytes (default: 32)

**Returns:** URL-safe base64 encoded string

```python
import secrets
token = secrets.token_urlsafe(16)
# "jlfB2t8xqbQ..."
```

### randbelow(n)

Generate a random integer in the range [0, n).

**Parameters:**

- `n` - Exclusive upper bound (must be positive)

**Returns:** Random integer from 0 to n-1

```python
import secrets
dice = secrets.randbelow(6) + 1  # Random 1-6
```

### choice(sequence)

Return a cryptographically random element from a non-empty sequence.

**Parameters:**

- `sequence` - Non-empty list to choose from

**Returns:** Random element from the sequence

```python
import secrets
winner = secrets.choice(["Alice", "Bob", "Charlie"])
```

### compare_digest(a, b)

Compare two strings using constant-time comparison to prevent timing attacks.

**Parameters:**

- `a` - First string
- `b` - Second string

**Returns:** True if strings are equal, False otherwise

```python
import secrets
# Secure comparison of secret values
is_valid = secrets.compare_digest(user_token, stored_token)
```

## Enabling in Go

```go
package main

import (
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/extlibs"
)

func main() {
    p := scriptling.New()

    // Register the secrets library
    p.RegisterLibrary("secrets", extlibs.SecretsLibrary)

    p.Eval(`
import secrets
token = secrets.token_hex(32)
print(token)
    `)
}
```

## Examples

### Generate an API Token

```python
import secrets

# Generate a 32-byte URL-safe token
api_token = secrets.token_urlsafe(32)
print(f"Your API token: {api_token}")
```

### Secure Password Reset Token

```python
import secrets

def generate_reset_token():
    return secrets.token_hex(32)

token = generate_reset_token()
# Store token with expiration time
```

### Validate a Token

```python
import secrets

def validate_token(user_token, stored_token):
    # Use constant-time comparison to prevent timing attacks
    return secrets.compare_digest(user_token, stored_token)

is_valid = validate_token(request_token, database_token)
```

### Random Selection

```python
import secrets

# Secure random selection (use instead of random.choice for security)
passcode_chars = "0123456789"
passcode = ""
for i in range(6):
    passcode = passcode + secrets.choice(list(passcode_chars))
print(passcode)  # Random 6-digit code
```

## When to Use

Use `secrets` instead of `random` when:

- Generating tokens, keys, or passwords
- Creating nonces or salts
- Any cryptographic or security-sensitive application
- Protecting against prediction attacks

Use `random` when:

- Performance matters more than security
- Statistical randomness is sufficient
- Non-security applications (games, simulations)

## Python Compatibility

This library implements Python's `secrets` module:

| Function           | Supported |
| ------------------ | --------- |
| token_bytes        | ✅        |
| token_hex          | ✅        |
| token_urlsafe      | ✅        |
| randbelow          | ✅        |
| choice             | ✅        |
| compare_digest     | ✅        |
| SystemRandom class | ❌        |
