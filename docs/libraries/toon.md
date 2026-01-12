# TOON Library

TOON (Token-Oriented Object Notation) encoding/decoding library. TOON is a human-readable data format that's more compact and readable than JSON, with support for inline objects, flexible syntax, and tabular data.

## Functions

### toon.encode(data)

Encodes a scriptling value to TOON format.

**Parameters:**
- `data`: Any scriptling value to encode (string, int, float, bool, list, dict)

**Returns:** str - TOON formatted string

**Example:**
```python
import toon

# Encode a dictionary
data = {"name": "Alice", "age": 30, "active": true}
text = toon.encode(data)
print(text)
# name: Alice
# age: 30
# active: true

# Encode a list
items = [1, 2, 3, 4, 5]
text = toon.encode(items)
print(text)
# - 1
# - 2
# - 3
# - 4
# - 5
```

### toon.decode(text)

Decodes a TOON formatted string to scriptling objects.

**Parameters:**
- `text` (str): TOON formatted string

**Returns:** object - Decoded scriptling value (dict, list, string, int, float, bool, null)

**Example:**
```python
import toon

text = """
name: Alice
age: 30
active: true
tags:
  - python
  - golang
"""

data = toon.decode(text)
print(data.name)    # "Alice"
print(data.age)     # 30
print(data.active)  # true
print(data.tags[0]) # "python"
```

### toon.encode_options(data, indent, delimiter)

Encodes data to TOON format with custom options.

**Parameters:**
- `data`: Any scriptling value to encode
- `indent` (int): Number of spaces per indentation level (default: 2)
- `delimiter` (str): Delimiter for arrays and tabular data (default: ",")

**Returns:** str - TOON formatted string

**Example:**
```python
import toon

data = [1, 2, 3, 4, 5]

# With custom indentation
text = toon.encode_options(data, 4, ",")
print(text)
# - 1
# - 2
# - 3
# - 4
# - 5
```

### toon.decode_options(text, strict, indent_size)

Decodes a TOON formatted string with custom parsing options.

**Parameters:**
- `text` (str): TOON formatted string
- `strict` (bool): Enable strict validation (default: true)
- `indent_size` (int): Expected indentation size (0 = auto-detect, default: 0)

**Returns:** object - Decoded scriptling value

**Example:**
```python
import toon

text = """
name: Alice
age: 30
"""

# Lenient parsing
data = toon.decode_options(text, false, 0)
```

## TOON Format Reference

### Basic Types

**Strings:**
```toon
name: Alice
description: Hello, World!
empty: ""
```

**Numbers:**
```toon
count: 42
price: 19.99
negative: -10
scientific: 1.5e-10
```

**Booleans:**
```toon
active: true
deleted: false
```

**Null:**
```toon
value: null
```

### Lists

Inline lists:
```toon
numbers: [1, 2, 3]
names: [Alice, Bob, Charlie]
mixed: [1, two, 3.0, true]
```

Multiline lists:
```toon
fruits:
  - apple
  - banana
  - orange
```

### Dictionaries

Inline dictionaries:
```toon
person: {name: Alice, age: 30}
```

Multiline dictionaries:
```toon
person:
  name: Alice
  age: 30
  city: Paris
```

Nested structures:
```toon
company:
  name: TechCorp
  employees:
    - name: Alice
      role: Developer
    - name: Bob
      role: Designer
  address:
    street: 123 Main St
    city: San Francisco
```

### Tabular Data

TOON supports tabular data with automatic detection:

```toon
users:
  name    | age   | city
  Alice   | 30    | Paris
  Bob     | 25    | London
  Charlie | 35    | New York
```

This decodes to:
```python
{
  "users": [
    {"name": "Alice", "age": 30, "city": "Paris"},
    {"name": "Bob", "age": 25, "city": "London"},
    {"name": "Charlie", "age": 35, "city": "New York"}
  ]
}
```

### Comments

TOON supports `#` comments:

```toon
# This is a comment
name: Alice  # Inline comment
age: 30

# Multi-line comment
# on multiple lines
active: true
```

## Usage Examples

### Configuration Files

```python
import toon

config_text = """
# Application Configuration
app:
  name: MyApp
  version: 1.0.0
  debug: true

server:
  host: localhost
  port: 8080
  ssl: false

database:
  driver: postgres
  host: db.example.com
  port: 5432
  name: myapp_db
"""

config = toon.decode(config_text)
print(config.app.name)        # "MyApp"
print(config.server.port)     # 8080
print(config.database.driver) # "postgres"
```

### Data Exchange

```python
import toon

# Encode complex data
data = {
  "users": [
    {"name": "Alice", "age": 30, "skills": ["Python", "Go"]},
    {"name": "Bob", "age": 25, "skills": ["JavaScript", "React"]}
  ],
  "metadata": {
    "count": 2,
    "updated": "2024-01-15"
  }
}

toon_text = toon.encode(data)
print(toon_text)
```

### Compact Syntax

```python
import toon

# TOON is more compact than JSON
data = {
  "name": "Alice",
  "age": 30,
  "active": true,
  "tags": ["python", "golang", "developer"]
}

# TOON output (more concise):
# name: Alice
# age: 30
# active: true
# tags: [python, golang, developer]

# JSON equivalent:
# {"name":"Alice","age":30,"active":true,"tags":["python","golang","developer"]}
```

## Comparing TOON to JSON

### TOON Advantages

1. **More readable:** No excessive braces and quotes
2. **Less verbose:** Optional quotes, flexible syntax
3. **Better for humans:** Easier to write and edit manually
4. **Tabular data:** Native support for tables
5. **Comments:** Built-in comment support

### Example Comparison

**JSON:**
```json
{
  "name": "Alice",
  "age": 30,
  "active": true,
  "tags": ["python", "golang"]
}
```

**TOON:**
```toon
name: Alice
age: 30
active: true
tags: [python, golang]
```

## Advanced Features

### Auto-Detection of Indent

```python
import toon

# TOON can auto-detect indentation
text = """
name: Alice
  address:
    street: 123 Main St
    city: Paris
"""

data = toon.decode(text)
```

### Custom Delimiters

```python
import toon

# Use different delimiter for arrays
data = [1, 2, 3, 4, 5]
text = toon.encode_options(data, 2, "|")
# Output: [1|2|3|4|5]
```

### Strict vs Lenient Parsing

```python
import toon

text = """
name: Alice
age: thirty  # Invalid number
"""

# Strict mode (default) - will fail on errors
try:
  data = toon.decode(text)
except Exception as e:
  print("Parse error:", e)

# Lenient mode - attempts to continue
data = toon.decode_options(text, false, 0)
```

## Error Handling

```python
import toon

try:
  # Invalid TOON syntax
  text = """
  name: Alice
    age: 30  # Wrong indentation
  """
  data = toon.decode(text)
except Exception as e:
  print("TOON parse error:", e)
```

## Best Practices

1. **Use 2-space indentation** for consistency
2. **Quote strings with special characters** (`:`, `[`, `]`, `{`, `}`, `#`)
3. **Use comments** to document complex structures
4. **Prefer multiline format** for complex objects
5. **Use inline format** for simple values
6. **Leverage tabular syntax** for uniform data

## When to Use TOON vs JSON

### Use TOON when:
- Writing configuration files by hand
- Human readability is important
- Data contains nested structures
- You need comments in your data
- Working with tabular data

### Use JSON when:
- Interoperating with other systems
- Machine parsing is prioritized
- Data is simple and flat
- Using existing JSON tools/libraries
