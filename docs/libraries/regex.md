# Regex Library

Regular expression functions for pattern matching and text processing. The function signatures follow Python's `re` module conventions.

## Constants (Flags)

The regex library provides the following flags that can be passed to functions:

| Flag | Shorthand | Value | Description |
|------|-----------|-------|-------------|
| `re.IGNORECASE` | `re.I` | 2 | Case-insensitive matching |
| `re.MULTILINE` | `re.M` | 8 | `^` and `$` match at line boundaries |
| `re.DOTALL` | `re.S` | 16 | `.` matches newlines |

Flags can be combined using the bitwise OR operator (`|`):

```python
import re

# Combine IGNORECASE and MULTILINE
re.match("hello", "HELLO\nWORLD", re.I | re.M)
```

## Functions

### re.match(pattern, string, flags=0)

Checks if the pattern matches at the beginning of the string.

**Parameters:**
- `pattern`: Regular expression pattern
- `string`: String to search
- `flags`: Optional flags (default: 0)

**Returns:** Boolean (True if matches at start, False otherwise)

**Example:**
```python
import re

if re.match("[0-9]+", "123abc"):
    print("String starts with digits")  # This prints

if re.match("[0-9]+", "abc123"):
    print("This won't print - pattern must match at start")

# Case-insensitive matching
if re.match("hello", "HELLO world", re.I):
    print("Case-insensitive match")  # This prints
```

### re.search(pattern, string, flags=0)

Searches for the first occurrence of the pattern anywhere in the string.

**Parameters:**
- `pattern`: Regular expression pattern
- `string`: String to search
- `flags`: Optional flags (default: 0)

**Returns:** String (matched text) or None if not found

**Example:**
```python
import re

email = re.search("\\w+@\\w+\\.\\w+", "Contact: user@example.com")
print(email)  # "user@example.com"

result = re.search("[0-9]+", "no numbers")
print(result)  # None

# Case-insensitive search
result = re.search("world", "HELLO WORLD", re.I)
print(result)  # "WORLD"
```

### re.findall(pattern, string, flags=0)

Finds all occurrences of the pattern in the string.

**Parameters:**
- `pattern`: Regular expression pattern
- `string`: String to search
- `flags`: Optional flags (default: 0)

**Returns:** List of strings (all matches)

**Example:**
```python
import re

phones = re.findall("[0-9]{3}-[0-9]{4}", "Call 555-1234 or 555-5678")
print(phones)  # ["555-1234", "555-5678"]

# Case-insensitive findall
words = re.findall("a+", "aAbBaAa", re.I)
print(words)  # ["aA", "aAa"]
```

### re.sub(pattern, repl, string, count=0, flags=0)

Replaces occurrences of the pattern in the string with the replacement. This follows Python's `re.sub()` function signature.

**Parameters:**
- `pattern`: Regular expression pattern
- `repl`: Replacement string
- `string`: String to modify
- `count`: Maximum number of replacements (0 = all, default: 0)
- `flags`: Optional flags (default: 0)

**Returns:** String (modified text)

**Example:**
```python
import re

text = re.sub("[0-9]+", "XXX", "Price: 100")
print(text)  # "Price: XXX"

# Replace multiple occurrences
result = re.sub("[0-9]+", "#", "a1b2c3")
print(result)  # "a#b#c#"

# Limit replacements with count
result = re.sub("[0-9]+", "X", "a1b2c3", 2)
print(result)  # "aXbXc3"

# Case-insensitive replacement
result = re.sub("hello", "hi", "Hello HELLO hello", 0, re.I)
print(result)  # "hi hi hi"
```

### re.split(pattern, string, maxsplit=0, flags=0)

Splits the string by occurrences of the pattern.

**Parameters:**
- `pattern`: Regular expression pattern
- `string`: String to split
- `maxsplit`: Maximum number of splits (0 = all, default: 0)
- `flags`: Optional flags (default: 0)

**Returns:** List of strings (split parts)

**Example:**
```python
import re

parts = re.split("[,;]", "one,two;three")
print(parts)  # ["one", "two", "three"]

# Limit splits
parts = re.split("[,;]", "a,b;c;d", 2)
print(parts)  # ["a", "b;c;d"]
```

### re.compile(pattern, flags=0)

Compiles a regular expression pattern for validation and caching.

**Parameters:**
- `pattern`: Regular expression pattern
- `flags`: Optional flags (default: 0)

**Returns:** String (the compiled pattern with flags applied) or error if invalid

**Example:**
```python
import re

pattern = re.compile("[0-9]+")  # Validates and caches the pattern
print(pattern)  # "[0-9]+"

# Compile with flags
pattern = re.compile("hello", re.I)
print(pattern)  # "(?i)hello"

# Compile with multiple flags
pattern = re.compile("hello", re.I | re.M)
print(pattern)  # "(?im)hello"
```

### re.escape(string)

Escapes special regex characters in a string.

**Parameters:**
- `string`: String to escape

**Returns:** String (escaped text)

**Example:**
```python
import re

escaped = re.escape("a.b+c")
print(escaped)  # "a\.b\+c"
```

### re.fullmatch(pattern, string, flags=0)

Checks if the pattern matches the entire string.

**Parameters:**
- `pattern`: Regular expression pattern
- `string`: String to match
- `flags`: Optional flags (default: 0)

**Returns:** Boolean (True if entire string matches, False otherwise)

**Example:**
```python
import re

if re.fullmatch("[0-9]+", "123"):
    print("Entire string is digits")  # This prints

if re.fullmatch("[0-9]+", "123abc"):
    print("This won't print - doesn't match entire string")

# Case-insensitive fullmatch
if re.fullmatch("hello", "HELLO", re.I):
    print("Case-insensitive full match")  # This prints
```

## Regular Expression Syntax

Scriptling uses Go's regexp syntax, which is similar to Perl/Python:

### Basic Patterns
- `.` - Any character (newlines only with DOTALL flag)
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
- `^` - Start of string (or line with MULTILINE flag)
- `$` - End of string (or line with MULTILINE flag)
- `\b` - Word boundary
- `\B` - Not word boundary

### Inline Flags
You can also use inline flag modifiers in your patterns:
- `(?i)` - Case-insensitive
- `(?m)` - Multiline mode
- `(?s)` - Dotall mode (. matches newlines)

## Usage Examples

```python
import re

# Basic matching at start of string
if re.match("[0-9]+", "123abc"):
    print("String starts with digits")

# Search anywhere in string
email = re.search("\\w+@\\w+\\.\\w+", "Contact: user@example.com")
# "user@example.com"

# Find all matches
numbers = re.findall("[0-9]+", "abc123def456")
# ["123", "456"]

# Replace text
text = re.sub("[0-9]+", "XXX", "Price: 100")
# "Price: XXX"

# Replace with count limit
text = re.sub("[0-9]+", "X", "1 2 3 4 5", 3)
# "X X X 4 5"

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

# Case-insensitive matching with flag
if re.match("hello", "HELLO world", re.I):
    print("Case-insensitive match")

# Case-insensitive matching with inline flag
if re.match("(?i)hello", "HELLO world"):
    print("Case-insensitive match")

# Multiline matching
text = "line1\nline2\nline3"
matches = re.findall("^line", text, re.M)
# ["line", "line", "line"]

# Dotall - dot matches newlines
result = re.search("a.*b", "a\nb", re.S)
# "a\nb"
```

## Notes

- Patterns use Go's regexp engine (RE2)
- All functions are case-sensitive by default
- Use `re.I` or `re.IGNORECASE` flag for case-insensitive matching
- Alternatively, use `(?i)` at the start of pattern for case-insensitive matching
- Backslashes in patterns need to be escaped in Scriptling strings
- The `count` parameter in `re.sub()` limits the number of replacements (0 = replace all)
- The `maxsplit` parameter in `re.split()` limits the number of splits
