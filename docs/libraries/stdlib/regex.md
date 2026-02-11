# Regex Library

Regular expression functions for pattern matching and text processing. The function signatures follow Python's `re` module conventions.

## Match Objects

The `re.match()` and `re.search()` functions return a Match object on success, or `None` if no match is found. Match objects provide the following methods:

| Method | Description |
|--------|-------------|
| `group(n=0)` | Returns the nth matched group (0 = full match) |
| `groups()` | Returns a tuple of all capturing groups (excluding group 0) |
| `start(n=0)` | Returns the start position of the match |
| `end(n=0)` | Returns the end position of the match |
| `span(n=0)` | Returns a (start, end) tuple for the match |

**Example:**
```python
import re

# Search with capturing groups
m = re.search(r'(\w+)@(\w+)\.(\w+)', 'Email: user@example.com')
if m:
    print(m.group(0))   # 'user@example.com' (full match)
    print(m.group(1))   # 'user' (first group)
    print(m.group(2))   # 'example' (second group)
    print(m.group(3))   # 'com' (third group)
    print(m.groups())   # ('user', 'example', 'com')
    print(m.start())    # 7 (position where match starts)
    print(m.end())      # 23 (position where match ends)
    print(m.span())     # (7, 23)
```

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
m = re.match("hello", "HELLO\nWORLD", re.I | re.M)
if m:
    print(m.group(0))  # "HELLO"
```

## Functions

### re.match(pattern, string, flags=0)

Checks if the pattern matches at the beginning of the string.

**Parameters:**
- `pattern`: Regular expression pattern
- `string`: String to search
- `flags`: Optional flags (default: 0)

**Returns:** Match object if pattern matches at start, or `None` if no match

**Example:**
```python
import re

m = re.match("[0-9]+", "123abc")
if m:
    print("String starts with digits:", m.group(0))  # "123"

m = re.match("[0-9]+", "abc123")
if m == None:
    print("Pattern must match at start")

# Case-insensitive matching
m = re.match("hello", "HELLO world", re.I)
if m:
    print("Case-insensitive match:", m.group(0))  # "HELLO"
```

### re.search(pattern, string, flags=0)

Searches for the first occurrence of the pattern anywhere in the string.

**Parameters:**
- `pattern`: Regular expression pattern
- `string`: String to search
- `flags`: Optional flags (default: 0)

**Returns:** Match object for the first match, or `None` if no match found

**Example:**
```python
import re

m = re.search(r'\w+@\w+\.\w+', "Contact: user@example.com")
if m:
    print(m.group(0))  # "user@example.com"

result = re.search("[0-9]+", "no numbers")
print(result)  # None

# Case-insensitive search
m = re.search("world", "HELLO WORLD", re.I)
if m:
    print(m.group(0))  # "WORLD"

# Using capturing groups
m = re.search(r'(\d+)-(\d+)', "Phone: 555-1234")
if m:
    print(m.group(0))  # "555-1234"
    print(m.group(1))  # "555"
    print(m.group(2))  # "1234"
    print(m.groups())  # ("555", "1234")
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

### re.finditer(pattern, string, flags=0)

Finds all occurrences of the pattern in the string and returns Match objects.

**Parameters:**
- `pattern`: Regular expression pattern
- `string`: String to search
- `flags`: Optional flags (default: 0)

**Returns:** List of Match objects (all matches)

**Example:**
```python
import re

matches = re.finditer("[0-9]{3}-[0-9]{4}", "Call 555-1234 or 555-5678")
for match in matches:
    print(match.group(0))  # "555-1234", "555-5678"
    print(match.start())   # 5, 18
    print(match.end())     # 13, 26

# With capturing groups
matches = re.finditer(r'(\d+)-(\d+)', "555-1234, 888-9999")
for match in matches:
    print(match.group(0))  # "555-1234", "888-9999"
    print(match.group(1))  # "555", "888"
    print(match.group(2))  # "1234", "9999"
    print(match.groups())  # ("555", "1234"), ("888", "9999")
```

### re.sub(pattern, repl, string, count=0, flags=0)

Replaces occurrences of the pattern in the string with the replacement. The replacement can be either a string or a function. This follows Python's `re.sub()` function signature.

**Parameters:**
- `pattern`: Regular expression pattern
- `repl`: Replacement string or function that takes a Match object and returns a string
- `string`: String to modify
- `count`: Maximum number of replacements (0 = all, default: 0)
- `flags`: Optional flags (default: 0)

**Returns:** String (modified text)

**Example:**
```python
import re

# String replacement
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

# Function replacement - uppercase all words
result = re.sub(r'(\w+)', lambda m: m.group(1).upper(), "hello world")
print(result)  # "HELLO WORLD"

# Function replacement - swap first and last name
result = re.sub(r'(\w+) (\w+)', lambda m: m.group(2) + " " + m.group(1), "John Doe")
print(result)  # "Doe John"

# Function replacement - format inline code
backtick = chr(96)
result = re.sub(backtick + r'([^' + backtick + r']+)' + backtick, 
                lambda m: "[" + m.group(1) + "]", 
                "test `code` here")
print(result)  # "test [code] here"
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

**Returns:** Regex object (compiled pattern) or error if invalid

**Example:**
```python
import re

pattern = re.compile("[0-9]+")  # Validates and caches the pattern
print(type(pattern))  # "Regex"

# Compile with flags
pattern = re.compile("hello", re.I)
print(type(pattern))  # "Regex"

# Compile with multiple flags
pattern = re.compile("hello", re.I | re.M)
print(type(pattern))  # "Regex"
```

#### Compiled Pattern Methods

The Regex object returned by `re.compile()` provides the following methods:

- `pattern.match(string)` - Match at start of string
- `pattern.search(string)` - Search anywhere in string
- `pattern.findall(string)` - Find all matches as strings
- `pattern.finditer(string)` - Find all matches as Match objects

**Example:**
```python
import re

pattern = re.compile(r'\d+')
m = pattern.match("123abc")  # Match at start
if m:
    print(m.group(0))  # "123"

matches = pattern.findall("a1b2c3")  # ["1", "2", "3"]

match_objects = pattern.finditer("a1b2c3")
for match in match_objects:
    print(match.group(0), match.start(), match.end())
    # "1" 1 2
    # "2" 3 4
    # "3" 5 6
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
m = re.match("[0-9]+", "123abc")
if m:
    print("String starts with:", m.group(0))  # "123"

# Search anywhere in string
m = re.search(r'\w+@\w+\.\w+', "Contact: user@example.com")
if m:
    print("Email:", m.group(0))  # "user@example.com"

# Search with groups
m = re.search(r'(\w+)@(\w+)\.(\w+)', "Contact: user@example.com")
if m:
    print("User:", m.group(1))    # "user"
    print("Domain:", m.group(2))  # "example"
    print("TLD:", m.group(3))     # "com"
    print("Groups:", m.groups())  # ("user", "example", "com")

# Find all matches
numbers = re.findall("[0-9]+", "abc123def456")
# ["123", "456"]

# Find all matches as Match objects
matches = re.finditer("[0-9]+", "abc123def456")
for match in matches:
    print(match.group(0), match.start(), match.end())
    # "123" 3 6
    # "456" 9 12

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
# Regex object

# Use compiled pattern
matches = pattern.finditer("abc123def456")
for match in matches:
    print(match.group(0))  # "123", "456"

# Escape special characters
escaped = re.escape("a.b+c*d?")
# "a\.b\+c\*d\?"

# Full match entire string
if re.fullmatch("[0-9]+", "123"):
    print("String contains only digits")

# Case-insensitive matching with flag
m = re.match("hello", "HELLO world", re.I)
if m:
    print("Case-insensitive match:", m.group(0))

# Case-insensitive matching with inline flag
m = re.match("(?i)hello", "HELLO world")
if m:
    print("Inline flag match:", m.group(0))

# Multiline matching
text = "line1\nline2\nline3"
matches = re.findall("^line", text, re.M)
# ["line", "line", "line"]

# Dotall - dot matches newlines
m = re.search("a.*b", "a\nb", re.S)
if m:
    print("Dotall match:", m.group(0))  # "a\nb"
```

## Notes

- Patterns use Go's regexp engine (RE2)
- `re.match()` and `re.search()` return Match objects (not strings) like Python
- All functions are case-sensitive by default
- Use `re.I` or `re.IGNORECASE` flag for case-insensitive matching
- Alternatively, use `(?i)` at the start of pattern for case-insensitive matching
- Backslashes in patterns need to be escaped in Scriptling strings
- The `count` parameter in `re.sub()` limits the number of replacements (0 = replace all)
- The `maxsplit` parameter in `re.split()` limits the number of splits