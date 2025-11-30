---
description: "Generate a secure random password with customizable length and character sets"
---

# Generating Secure Passwords

This skill shows how to generate a secure, random password using Scriptling's random and string modules.

## Features

- Customizable password length (default 12)
- Option to include uppercase, lowercase, digits, and punctuation
- Ensures at least one character from each selected set for better security

## Scriptling Code

```python
import random
import string

def generate_password(length=12, use_upper=True, use_lower=True, use_digits=True, use_punct=False):
    """
    Generate a secure random password.

    Parameters:
    - length: Password length (default 12)
    - use_upper: Include uppercase letters
    - use_lower: Include lowercase letters
    - use_digits: Include digits
    - use_punct: Include punctuation
    """
    # Build character set
    chars = ""
    if use_lower:
        chars += string.ascii_lowercase
    if use_upper:
        chars += string.ascii_uppercase
    if use_digits:
        chars += string.digits
    if use_punct:
        chars += string.punctuation

    if not chars:
        return "Error: No character sets selected"

    # Ensure at least one of each selected type
    password = []
    if use_lower:
        password.append(random.choice(string.ascii_lowercase))
    if use_upper:
        password.append(random.choice(string.ascii_uppercase))
    if use_digits:
        password.append(random.choice(string.digits))
    if use_punct:
        password.append(random.choice(string.punctuation))

    # Fill the rest randomly
    while len(password) < length:
        password.append(random.choice(chars))

    # Shuffle to avoid predictable patterns
    random.shuffle(password)

    return "".join(password)

# Example usage
password = generate_password(16, True, True, True, True)
print("Generated password:", password)
print("Length:", len(password))
```

## Security Notes

- Uses cryptographically secure random number generation
- Includes characters from multiple sets for complexity
- Shuffles the password to avoid predictable ordering
- Recommended minimum length is 12 characters