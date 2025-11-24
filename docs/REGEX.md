# Regular Expression Library

## Overview

The `regex` library provides pattern matching and text processing capabilities using Go's standard `regexp` package.

## Functions

### re.match(pattern, text)

Check if a pattern matches anywhere in the text.

**Returns:** Boolean (True if match found, False otherwise)

```python
import re

if re.match("[0-9]+", "abc123"):
    print("Contains digits")

if re.match("^[a-z]+$", "hello"):
    print("All lowercase letters")
```

### re.find(pattern, text)

Find the first occurrence of a pattern in text.

**Returns:** String (first match) or None (if no match)

```python
import re

# Extract email
email = re.find("[a-z]+@[a-z]+\\.[a-z]+", "Contact: user@example.com")
print(email)  # "user@example.com"

# No match returns None
result = re.find("[0-9]+", "no digits here")
if result == None:
    print("No match found")
```

### re.findall(pattern, text)

Find all occurrences of a pattern in text.

**Returns:** List of strings (all matches)

```python
import re

# Find all phone numbers
phones = re.findall("[0-9]{3}-[0-9]{4}", "Call 555-1234 or 555-5678")
print(phones)  # ["555-1234", "555-5678"]

# Find all words
words = re.findall("[a-zA-Z]+", "Hello, World! 123")
print(words)  # ["Hello", "World"]
```

### re.replace(pattern, text, replacement)

Replace all occurrences of a pattern with a replacement string.

**Returns:** String (modified text)

```python
import re

# Redact numbers
text = re.replace("[0-9]+", "Price: 100 and 200", "XXX")
print(text)  # "Price: XXX and XXX"

# Remove extra spaces
text = re.replace(" +", "too  many   spaces", " ")
print(text)  # "too many spaces"
```

### re.split(pattern, text)

Split text by a pattern.

**Returns:** List of strings (split parts)

```python
import re

# Split by multiple delimiters
parts = re.split("[,;:]", "one,two;three:four")
print(parts)  # ["one", "two", "three", "four"]

# Split by whitespace
words = re.split("\\s+", "hello   world  test")
print(words)  # ["hello", "world", "test"]
```

## Common Patterns

### Email Validation
```python
email_pattern = "[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}"
if re.match(email_pattern, user_input):
    print("Valid email format")
```

### Phone Numbers
```python
# US phone format: 555-1234
phone = re.find("[0-9]{3}-[0-9]{4}", text)

# With area code: (555) 123-4567
phone = re.find("\\([0-9]{3}\\) [0-9]{3}-[0-9]{4}", text)
```

### URLs
```python
url = re.find("https?://[a-zA-Z0-9.-]+\\.[a-z]{2,}", text)
```

### Extract Numbers
```python
numbers = re.findall("[0-9]+", "abc123def456")
# ["123", "456"]
```

### Clean Whitespace
```python
# Remove leading/trailing spaces
text = re.replace("^\\s+|\\s+$", text, "")

# Collapse multiple spaces
text = re.replace("\\s+", text, " ")
```

## Pattern Syntax

Scriptling uses Go's regexp syntax (similar to Python's `re` module):

- `.` - Any character
- `^` - Start of string
- `$` - End of string
- `*` - Zero or more
- `+` - One or more
- `?` - Zero or one
- `[abc]` - Character class
- `[^abc]` - Negated character class
- `[a-z]` - Character range
- `\\d` - Digit (same as `[0-9]`)
- `\\w` - Word character (same as `[a-zA-Z0-9_]`)
- `\\s` - Whitespace
- `{n}` - Exactly n times
- `{n,}` - n or more times
- `{n,m}` - Between n and m times
- `(...)` - Grouping
- `|` - Alternation

## Examples

### Validate Input
```python
import re

def validate_username(username):
    # 3-16 alphanumeric characters
    if re.match("^[a-zA-Z0-9]{3,16}$", username):
        return True
    return False

def validate_password(password):
    # At least 8 chars, must have digit
    if len(password) >= 8 and re.match("[0-9]", password):
        return True
    return False
```

### Parse Log Files
```python
import re

log_line = "2024-01-15 10:30:45 ERROR: Connection failed"

# Extract date
date = re.find("[0-9]{4}-[0-9]{2}-[0-9]{2}", log_line)

# Extract time
time = re.find("[0-9]{2}:[0-9]{2}:[0-9]{2}", log_line)

# Extract level
level = re.find("ERROR|WARN|INFO", log_line)
```

### Clean Data
```python
import re

def clean_text(text):
    # Remove special characters
    text = re.replace("[^a-zA-Z0-9\\s]", text, "")
    
    # Collapse whitespace
    text = re.replace("\\s+", text, " ")
    
    return text

result = clean_text("Hello!!!  World???  123")
# "Hello World 123"
```

## Best Practices

1. **Test patterns** - Verify regex patterns work as expected
2. **Escape special characters** - Use `\\` to escape `.`, `*`, `+`, etc.
3. **Use raw strings** - In patterns, remember to escape backslashes
4. **Check for None** - `find()` returns None when no match found
5. **Validate input** - Use regex for validation before processing

## Performance

- Patterns are compiled on each call
- For repeated use, consider caching results
- Simple patterns are fast, complex patterns may be slower
- `findall()` processes entire string, may be slower on large text
