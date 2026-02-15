# textwrap Library

The `textwrap` library provides text wrapping and filling utilities, compatible with Python's `textwrap` module.

## Import

```python
import textwrap
```

## Available Functions

| Function               | Description                             |
| ---------------------- | --------------------------------------- |
| `wrap(text, width=70)` | Wrap text into list of lines            |
| `fill(text, width=70)` | Wrap text and join with newlines        |
| `dedent(text)`         | Remove common leading whitespace        |
| `indent(text, prefix)` | Add prefix to each line                 |
| `shorten(text, width)` | Truncate text to width with placeholder |

## Functions

### wrap(text, width=70)

Wrap a single paragraph of text, returning a list of wrapped lines.

**Parameters:**

- `text` - The text to wrap
- `width` - Maximum line width (default: 70)

**Returns:** List of lines

```python
import textwrap

text = "This is a long line of text that needs to be wrapped"
lines = textwrap.wrap(text, 20)
# ["This is a long line", "of text that needs", "to be wrapped"]
```

### fill(text, width=70)

Wrap text and return a single string with lines joined by newlines.

**Parameters:**

- `text` - The text to wrap
- `width` - Maximum line width (default: 70)

**Returns:** Wrapped text as a single string

```python
import textwrap

text = "This is a long line of text that needs to be wrapped"
result = textwrap.fill(text, 20)
# "This is a long line\nof text that needs\nto be wrapped"
```

### dedent(text)

Remove common leading whitespace from all lines in text.

**Parameters:**

- `text` - The text to dedent

**Returns:** Dedented text

```python
import textwrap

text = "    Hello\n    World\n    Test"
result = textwrap.dedent(text)
# "Hello\nWorld\nTest"
```

### indent(text, prefix)

Add a prefix to the beginning of non-empty lines in text.

**Parameters:**

- `text` - The text to indent
- `prefix` - String to add to each line

**Returns:** Indented text

```python
import textwrap

text = "Hello\nWorld"
result = textwrap.indent(text, "  ")
# "  Hello\n  World"

# Quote style
result = textwrap.indent(text, "> ")
# "> Hello\n> World"
```

### shorten(text, width, placeholder="[...]")

Truncate text to fit in the given width, using a placeholder.

**Parameters:**

- `text` - The text to shorten
- `width` - Maximum width including placeholder
- `placeholder` - String to indicate truncation (default: "[...]")

**Returns:** Shortened text

```python
import textwrap

text = "Hello World, this is a very long line of text"
result = textwrap.shorten(text, 20)
# "Hello World,[...]"

# Custom placeholder
result = textwrap.shorten(text, 20, placeholder="...")
# "Hello World,..."
```

## Examples

### Formatting Long Text

```python
import textwrap

article = "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."

# Wrap to 40 characters
wrapped = textwrap.fill(article, 40)
print(wrapped)
```

### Creating Indented Blocks

```python
import textwrap

code = "x = 1\ny = 2\nresult = x + y"
indented = textwrap.indent(code, "    ")
# "    x = 1\n    y = 2\n    result = x + y"
```

### Processing Multi-line Strings

```python
import textwrap

# Remove common indentation
template = """
    This text has
    common indentation
    that we want to remove
"""
clean = textwrap.dedent(template)
```

### Truncating for Display

```python
import textwrap

titles = [
    "A Short Title",
    "A Very Long Title That Needs To Be Truncated For Display"
]

for title in titles:
    display = textwrap.shorten(title, 30)
    print(display)
```

## Notes

- `wrap()` and `fill()` collapse whitespace and break at word boundaries
- `dedent()` only removes common leading whitespace (spaces/tabs that appear at the start of every line)
- `indent()` does not add prefix to empty lines by default
- `shorten()` attempts to break at word boundaries when truncating

## Python Compatibility

This library implements a subset of Python's `textwrap` module:

| Function          | Supported |
| ----------------- | --------- |
| wrap              | ✅        |
| fill              | ✅        |
| dedent            | ✅        |
| indent            | ✅        |
| shorten           | ✅        |
| TextWrapper class | ❌        |
