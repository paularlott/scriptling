# html Library

HTML escaping and unescaping library, matching Python's `html` module.

## Import

```python
import html
```

## Functions

### `escape(s)`
Escape HTML special characters in a string.

Converts:
- `&` → `&amp;`
- `<` → `&lt;`
- `>` → `&gt;`
- `"` → `&quot;`
- `'` → `&#39;`

**Parameters:**
- `s` - String to escape

**Returns:** Escaped string

```python
safe = html.escape("<script>alert('xss')</script>")
print(safe)  # "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"
```

### `unescape(s)`
Unescape HTML entities in a string.

Converts HTML entities back to their corresponding characters. Handles:
- Named entities: `&lt;`, `&gt;`, `&amp;`, `&quot;`, `&#39;`
- Numeric entities: `&#60;`, `&#x3c;`

**Parameters:**
- `s` - String with HTML entities to unescape

**Returns:** Unescaped string

```python
text = html.unescape("&lt;script&gt;")
print(text)  # "<script>"
```

## Examples

### Sanitize User Input
```python
import html

user_input = "<script>alert('xss')</script>"
safe_output = html.escape(user_input)
print(safe_output)
# Output: &lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;
```

### Build Safe HTML
```python
import html

def create_element(tag, content):
    safe_content = html.escape(content)
    return "<" + tag + ">" + safe_content + "</" + tag + ">"

print(create_element("p", "Hello <world>"))
# Output: <p>Hello &lt;world&gt;</p>
```

### Process HTML Entities
```python
import html

# From API response
encoded = "Tom &amp; Jerry"
decoded = html.unescape(encoded)
print(decoded)  # "Tom & Jerry"
```

### Roundtrip Conversion
```python
import html

original = '<div class="test">Content</div>'
escaped = html.escape(original)
restored = html.unescape(escaped)

print(original == restored)  # True
```

## Python Compatibility

- `escape(s)` - ✅ Compatible
- `unescape(s)` - ✅ Compatible

Note: Python's `html.escape()` has an optional `quote` parameter (default `True`) which is not implemented. Our implementation always escapes quotes.
