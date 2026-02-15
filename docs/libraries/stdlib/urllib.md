# urllib.parse Library

URL parsing and encoding/decoding functions. Python-compatible with the `urllib.parse` module.

```python
import urllib.parse
```

## Available Functions

| Function                    | Description                                |
| --------------------------- | ------------------------------------------ |
| `quote(string, safe?)`      | URL-encode a string using percent-encoding |
| `unquote(string)`           | Decode a URL-encoded string                |
| `quote_plus(string, safe?)` | URL-encode with spaces as plus signs       |
| `unquote_plus(string)`      | Decode plus signs to spaces                |
| `urlencode(dict)`           | Convert dict to URL query string           |
| `parse_qs(string)`          | Parse query string to dict                 |

## Functions

### urllib.parse.quote(string, safe?)

URL-encode a string using percent-encoding.

**Parameters:**

- `string`: String to encode
- `safe` (optional): String of characters that should not be encoded (default: "/")

**Returns:** String

**Example:**

```python
import urllib.parse

# Basic encoding
encoded = urllib.parse.quote("hello world")
# Returns: "hello%20world"

# With safe characters
encoded = urllib.parse.quote("hello/world", safe="")
# Returns: "hello%2Fworld"
```

### urllib.parse.unquote(string)

Decode a URL-encoded string.

**Parameters:**

- `string`: URL-encoded string to decode

**Returns:** String

**Example:**

```python
import urllib.parse

decoded = urllib.parse.unquote("hello%20world")
# Returns: "hello world"
```

### urllib.parse.quote_plus(string, safe?)

Like `quote()`, but also replaces spaces with plus signs.

**Parameters:**

- `string`: String to encode
- `safe` (optional): String of characters that should not be encoded (default: "")

**Returns:** String

**Example:**

```python
import urllib.parse

# Spaces become plus signs
encoded = urllib.parse.quote_plus("hello world")
# Returns: "hello+world"
```

### urllib.parse.unquote_plus(string)

Like `unquote()`, but also replaces plus signs with spaces.

**Parameters:**

- `string`: URL-encoded string to decode

**Returns:** String

**Example:**

```python
import urllib.parse

decoded = urllib.parse.unquote_plus("hello+world")
# Returns: "hello world"
```

### urllib.parse.urlparse(url)

Parse a URL into its components.

**Parameters:**

- `url`: URL string to parse

**Returns:** Dict with keys:

- `scheme`: Protocol (e.g., "https")
- `netloc`: Network location (host:port)
- `path`: URL path
- `params`: URL parameters
- `query`: Query string
- `fragment`: URL fragment

**Example:**

```python
import urllib.parse

parsed = urllib.parse.urlparse("https://example.com:8080/path?query=value#section")
# Returns: {
#   "scheme": "https",
#   "netloc": "example.com:8080",
#   "path": "/path",
#   "params": "",
#   "query": "query=value",
#   "fragment": "section"
# }
```

### urllib.parse.urlunparse(components)

Reconstruct a URL from its components.

**Parameters:**

- `components`: Dict or list with URL components (scheme, netloc, path, params, query, fragment)

**Returns:** String

**Example:**

```python
import urllib.parse

url = urllib.parse.urlunparse({
    "scheme": "https",
    "netloc": "example.com",
    "path": "/path",
    "query": "key=value"
})
# Returns: "https://example.com/path?key=value"

# Or as a list
url = urllib.parse.urlunparse(["https", "example.com", "/path", "", "key=value", ""])
# Returns: "https://example.com/path?key=value"
```

### urllib.parse.urlsplit(url)

Parse a URL into 5 components (without params).

**Parameters:**

- `url`: URL string to parse

**Returns:** Dict with keys:

- `scheme`: Protocol
- `netloc`: Network location
- `path`: URL path
- `query`: Query string
- `fragment`: URL fragment

**Example:**

```python
import urllib.parse

parsed = urllib.parse.urlsplit("https://example.com/path?query=value#section")
# Returns: {
#   "scheme": "https",
#   "netloc": "example.com",
#   "path": "/path",
#   "query": "query=value",
#   "fragment": "section"
# }
```

### urllib.parse.urlunsplit(components)

Reconstruct a URL from 5 components.

**Parameters:**

- `components`: Dict or list with 5 URL components

**Returns:** String

**Example:**

```python
import urllib.parse

url = urllib.parse.urlunsplit({
    "scheme": "https",
    "netloc": "example.com",
    "path": "/path",
    "query": "key=value",
    "fragment": "section"
})
# Returns: "https://example.com/path?key=value#section"
```

### urllib.parse.urljoin(base, url)

Join a base URL with another URL.

**Parameters:**

- `base`: Base URL
- `url`: URL to join (relative or absolute)

**Returns:** String

**Example:**

```python
import urllib.parse

# Relative path
full_url = urllib.parse.urljoin("https://example.com/path/", "page.html")
# Returns: "https://example.com/path/page.html"

# Absolute path
full_url = urllib.parse.urljoin("https://example.com/path/", "/other")
# Returns: "https://example.com/other"
```

### urllib.parse.parse_qs(query_string, keep_blank_values?)

Parse a query string into a dict with lists of values.

**Parameters:**

- `query_string`: Query string to parse
- `keep_blank_values` (optional): If true, keep blank values (default: false)

**Returns:** Dict with lists of values

**Example:**

```python
import urllib.parse

params = urllib.parse.parse_qs("name=John&tags=python&tags=go")
# Returns: {"name": ["John"], "tags": ["python", "go"]}
```

### urllib.parse.parse_qsl(query_string, keep_blank_values?)

Parse a query string into a list of (key, value) pairs.

**Parameters:**

- `query_string`: Query string to parse
- `keep_blank_values` (optional): If true, keep blank values (default: false)

**Returns:** List of [key, value] pairs

**Example:**

```python
import urllib.parse

params = urllib.parse.parse_qsl("name=John&age=30")
# Returns: [["name", "John"], ["age", "30"]]
```

### urllib.parse.urlencode(query, doseq?)

Encode a dict into a URL query string.

**Parameters:**

- `query`: Dict to encode
- `doseq` (optional): If true, handle sequences of values (default: false)

**Returns:** String

**Example:**

```python
import urllib.parse

query = urllib.parse.urlencode({"name": "John", "age": "30"})
# Returns: "name=John&age=30"

# With sequences
query = urllib.parse.urlencode({"tags": ["python", "go"]}, True)
# Returns: "tags=python&tags=go"
```

## Notes

- Use `urllib.parse` for Python compatibility
- All functions follow Python's urllib.parse signatures and behavior
- `netloc` includes userinfo (user:pass@host:port) matching Python's behavior
