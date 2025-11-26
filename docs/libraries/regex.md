# Regex Library

Regular expression functions for pattern matching and text processing.

## Functions

### re.match(pattern, text)

Checks if the pattern matches at the beginning of the text.

**Parameters:**
- `pattern`: Regular expression pattern
- `text`: Text to search

**Returns:** Boolean (True if matches, False otherwise)

**Example:**
```python
import re

if re.match("[0-9]+", "abc123"):
    print("Text starts with digits")
```

### re.find(pattern, text)

Finds the first occurrence of the pattern in the text.

**Parameters:**
- `pattern`: Regular expression pattern
- `text`: Text to search

**Returns:** String (matched text) or None if not found

**Example:**
```python
import re

email = re.find("\\w+@\\w+\\.\\w+", "Contact: user@example.com")
print(email)  # "user@example.com"
```

### re.findall(pattern, text)

Finds all occurrences of the pattern in the text.

**Parameters:**
- `pattern`: Regular expression pattern
- `text`: Text to search

**Returns:** List of strings (all matches)

**Example:**
```python
import re

phones = re.findall("[0-9]{3}-[0-9]{4}", "Call 555-1234 or 555-5678")
print(phones)  # ["555-1234", "555-5678"]
```

### re.replace(pattern, text, replacement)

Replaces all occurrences of the pattern in the text with the replacement.

**Parameters:**
- `pattern`: Regular expression pattern
- `text`: Text to search
- `replacement`: Replacement text

**Returns:** String (modified text)

**Example:**
```python
import re

text = re.replace("[0-9]+", "Price: 100", "XXX")
print(text)  # "Price: XXX"
```

### re.split(pattern, text)

Splits the text by occurrences of the pattern.

**Parameters:**
- `pattern`: Regular expression pattern
- `text`: Text to split

**Returns:** List of strings (split parts)

**Example:**
```python
import re

parts = re.split("[,;]", "one,two;three")
print(parts)  # ["one", "two", "three"]
```

### re.search(pattern, text)

Finds the first occurrence of the pattern anywhere in the text.

**Parameters:**
- `pattern`: Regular expression pattern
- `text`: Text to search

**Returns:** String (matched text) or None if not found

**Example:**
```python
import re

# Extract email
email = re.find("\\w+@\\w+\\.\\w+", "Contact: user@example.com")
# "user@example.com"
```

### re.compile(pattern)

Compiles a regular expression pattern for validation.

**Parameters:**
- `pattern`: Regular expression pattern

**Returns:** String (the pattern) or error if invalid

**Example:**
```python
import re

pattern = re.compile("[0-9]+")  # Validates and caches the pattern
print(pattern)  # "[0-9]+"
```

### re.escape(text)

Escapes special regex characters in text.

**Parameters:**
- `text`: Text to escape

**Returns:** String (escaped text)

**Example:**
```python
import re

escaped = re.escape("a.b+c")
print(escaped)  # "a\.b\+c"
```

### re.fullmatch(pattern, text)

Checks if the pattern matches the entire text.

**Parameters:**
- `pattern`: Regular expression pattern
- `text`: Text to match

**Returns:** Boolean (True if entire text matches, False otherwise)

**Example:**
```python
import re

if re.fullmatch("[0-9]+", "123"):
    print("Entire string is digits")
```

## Regular Expression Syntax

Scriptling uses Go's regexp syntax, which is similar to Perl/Python:

### Basic Patterns
- `.` - Any character
- `\d` - Digit (0-9)
- `\D` - Non-digit
- `\w` - Word character (a-z, A-Z, 0-9, _)
- `\W` - Non-word character
- `\s` - Whitespace
- `\S` - Non-whitespace

### Quantifiers
- `*` - Zero or more
- `+` - One or more
- `?` - Zero or one
- `{n}` - Exactly n times
- `{n,}` - n or more times
- `{n,m}` - Between n and m times

### Character Classes
- `[abc]` - Any of a, b, or c
- `[^abc]` - Not a, b, or c
- `[a-z]` - Any lowercase letter
- `[A-Z]` - Any uppercase letter
- `[0-9]` - Any digit

### Anchors
- `^` - Start of string/line
- `$` - End of string/line
- `\b` - Word boundary
- `\B` - Not word boundary

## Usage Examples

```python
import re

# Basic matching
if re.match("^\d{3}-\d{4}$", "555-1234"):
    print("Valid phone number")

# Find vs Search (both work the same in Scriptling)
email = re.find("[a-z]+@[a-z]+\.[a-z]+", "Contact: user@example.com")
# "user@example.com"

email = re.search("\\w+@\\w+\\.\\w+", "Contact: user@example.com")
# "user@example.com"

# Find all matches
numbers = re.findall("[0-9]+", "abc123def456")
# ["123", "456"]

# Replace text
text = re.replace("[0-9]+", "Price: 100", "XXX")
# "Price: XXX"

# Split by pattern
parts = re.split("[,;]", "one,two;three")
# ["one", "two", "three"]

# Compile pattern (validates and caches)
pattern = re.compile("[0-9]+")
# "[0-9]+"

# Escape special characters
escaped = re.escape("a.b+c*d?")
# "a\.b\+c\*d\?"

# Full match entire string
if re.fullmatch("[0-9]+", "123"):
    print("String contains only digits")

# Extract words
words = re.findall("\\b\\w+\\b", "Hello, world! How are you?")
# ["Hello", "world", "How", "are", "you"]
```

## Notes

- Patterns use Go's regexp engine
- All functions are case-sensitive by default
- Use `(?i)` at the start of pattern for case-insensitive matching
- Backslashes in patterns need to be escaped in Scriptling strings