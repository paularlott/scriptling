# URL Library

Functions for URL parsing, encoding, and manipulation.

## Functions

### url.encode(string)

URL-encodes a string (percent encoding).

**Parameters:**
- `string`: String to encode

**Returns:** String (URL-encoded)

**Example:**
```python
import url

encoded = url.encode("hello world!")
print(encoded)  # "hello%20world%21"
```

### url.decode(string)

URL-decodes a string.

**Parameters:**
- `string`: URL-encoded string to decode

**Returns:** String (decoded)

**Example:**
```python
import url

decoded = url.decode("hello%20world%21")
print(decoded)  # "hello world!"
```

### url.parse(string)

Parses a URL string into components.

**Parameters:**
- `string`: URL string to parse

**Returns:** Dictionary with URL components

**Example:**
```python
import url

parsed = url.parse("https://user:pass@example.com:8080/path?query=value#fragment")
print(parsed["scheme"])    # "https"
print(parsed["host"])      # "example.com:8080"
print(parsed["path"])      # "/path"
print(parsed["query"])     # "query=value"
print(parsed["fragment"])  # "fragment"
```

### url.build(components)

Builds a URL string from components.

**Parameters:**
- `components`: Dictionary with URL components

**Returns:** String (complete URL)

**Example:**
```python
import url

components = {
    "scheme": "https",
    "host": "example.com",
    "path": "/api/users",
    "query": "limit=10&offset=0"
}

url_str = url.build(components)
print(url_str)  # "https://example.com/api/users?limit=10&offset=0"
```

### url.query_parse(string)

Parses URL query string into a dictionary.

**Parameters:**
- `string`: Query string (with or without leading ?)

**Returns:** Dictionary of query parameters

**Example:**
```python
import url

query = url.query_parse("name=Alice&age=30&city=New%20York")
print(query["name"])  # "Alice"
print(query["age"])   # "30"
print(query["city"])  # "New York"
```

### url.join(base, path)

Joins a base URL with a relative path.

**Parameters:**
- `base`: Base URL
- `path`: Path to join

**Returns:** String (joined URL)

**Example:**
```python
import url

joined = url.join("https://api.example.com", "/users/123")
print(joined)  # "https://api.example.com/users/123"
```

### url.urlsplit(string)

Splits a URL into its components as a tuple/list.

**Parameters:**
- `string`: URL string to split

**Returns:** List with 5 elements: [scheme, netloc, path, query, fragment]

**Example:**
```python
import url

parts = url.urlsplit("https://example.com/path?query=value#fragment")
print(parts[0])  # "https" (scheme)
print(parts[1])  # "example.com" (netloc/host)
print(parts[2])  # "/path" (path)
print(parts[3])  # "query=value" (query)
print(parts[4])  # "fragment" (fragment)
```

### url.urlunsplit(parts)

Builds a URL string from a list of components.

**Parameters:**
- `parts`: List with exactly 5 string elements: [scheme, netloc, path, query, fragment]

**Returns:** String (complete URL)

**Example:**
```python
import url

parts = ["https", "example.com", "/api/users", "id=123&active=true", ""]
url_str = url.urlunsplit(parts)
print(url_str)  # "https://example.com/api/users?id=123&active=true"
```

### url.parse_qs(string)

Parses URL query string into a dictionary with list values.

**Parameters:**
- `string`: Query string (with or without leading ?)

**Returns:** Dictionary where each key maps to a list of values

**Example:**
```python
import url

query = url.parse_qs("name=Alice&name=Bob&age=30")
print(query["name"])  # ["Alice", "Bob"]
print(query["age"])   # ["30"]
```

### url.urlencode(data)

Encodes a dictionary into a URL query string.

**Parameters:**
- `data`: Dictionary where values can be strings or lists of strings

**Returns:** String (URL-encoded query string)

**Example:**
```python
import url

# Simple key-value pairs
data = {"name": "Alice", "age": "30", "city": "New York"}
query = url.urlencode(data)
print(query)  # "age=30&city=New+York&name=Alice"

# Multiple values for same key
data = {"tags": ["python", "web", "api"]}
query = url.urlencode(data)
print(query)  # "tags=python&tags=web&tags=api"
```

### url.path_join(...parts)

Joins path segments with forward slashes, properly handling leading/trailing slashes.

**Parameters:**
- `...parts`: Variable number of path segments to join

**Returns:** String (joined path)

**Example:**
```python
import url

path = url.path_join("api", "v1", "users", "123")
print(path)  # "api/v1/users/123"

# Handles slashes properly
path = url.path_join("/api/", "/v1/", "/users/")
print(path)  # "/api/v1/users"
```

## Usage Examples

```python
import url

# Encoding/Decoding
original = "Hello, World! How are you?"
encoded = url.encode(original)
print("Encoded:", encoded)

decoded = url.decode(encoded)
print("Decoded:", decoded)
print("Match:", original == decoded)

# URL parsing
url_str = "https://user:password@example.com:8080/api/users?id=123&active=true"
parsed = url.parse(url_str)

print("Scheme:", parsed["scheme"])      # "https"
print("Host:", parsed["host"])          # "example.com:8080"
print("Path:", parsed["path"])          # "/api/users"
print("Raw Query:", parsed["query"])    # "id=123&active=true"

# Query parsing
query_dict = url.query_parse(parsed["query"])
print("ID:", query_dict["id"])          # "123"
print("Active:", query_dict["active"])  # "true"

# URL building
new_url = url.build({
    "scheme": "https",
    "host": "api.example.com",
    "path": "/v1/users",
    "query": "limit=50&sort=name"
})
print("Built URL:", new_url)  # "https://api.example.com/v1/users?limit=50&sort=name"

# URL joining
base = "https://api.example.com/v1"
full_url = url.join(base, "/users/profile")
print("Joined URL:", full_url)  # "https://api.example.com/v1/users/profile"
```