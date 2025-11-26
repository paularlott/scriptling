# HTTP Library

Functions for making HTTP requests. Requires manual registration.

## Setup

```go
import "github.com/paularlott/scriptling/extlibs"

// Register the HTTP library
p.RegisterLibrary("http", extlibs.HTTPLibrary())
```

## Functions

### http.get(url, options?)

Makes a GET request to the specified URL.

**Parameters:**
- `url`: URL to request
- `options` (optional): Dictionary of request options

**Returns:** HTTP response object

### http.post(url, body, options?)

Makes a POST request to the specified URL.

**Parameters:**
- `url`: URL to request
- `body`: Request body (string)
- `options` (optional): Dictionary of request options

**Returns:** HTTP response object

### http.put(url, body, options?)

Makes a PUT request to the specified URL.

**Parameters:**
- `url`: URL to request
- `body`: Request body (string)
- `options` (optional): Dictionary of request options

**Returns:** HTTP response object

### http.delete(url, options?)

Makes a DELETE request to the specified URL.

**Parameters:**
- `url`: URL to request
- `options` (optional): Dictionary of request options

**Returns:** HTTP response object

### http.patch(url, body, options?)

Makes a PATCH request to the specified URL.

**Parameters:**
- `url`: URL to request
- `body`: Request body (string)
- `options` (optional): Dictionary of request options

**Returns:** HTTP response object

## Response Object

All HTTP functions return a response object with these attributes:

- `status_code` or `["status_code"]`: HTTP status code (integer)
- `text` or `["text"]`: Response body (string)
- `headers` or `["headers"]`: Response headers (dictionary)

## Response Methods

- `json()`: Parse response body as JSON
- `raise_for_status()`: Raise exception if status code >= 400

## Options

Request options dictionary:

- `timeout` (integer): Request timeout in seconds (default: 5)
- `headers` (dictionary): HTTP headers to send

## Examples

### Basic GET Request

```python
import http

response = http.get("https://api.example.com/users/1")
if response.status_code == 200:
    print("User data:", response.text)
```

### GET with Options

```python
import http

options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123"}
}
response = http.get("https://api.example.com/users/1", options)
```

### POST Request

```python
import http
import json

new_user = {"name": "Alice", "email": "alice@example.com"}
body = json.stringify(new_user)

response = http.post("https://api.example.com/users", body)
if response.status_code == 201:
    created = response.json()
    print("Created user:", created["id"])
```

### Error Handling

```python
import http

try:
    response = http.get("https://api.example.com/data")
    response.raise_for_status()  # Raises error if 4xx or 5xx
    data = response.json()
    print("Success:", data)
except Exception as e:
    print("Request failed:", e)
```

### Using Response Attributes

```python
import http

response = http.get("https://api.example.com/data")

# Both syntaxes work
status = response.status_code
# or
status = response["status_code"]

content = response.text
headers = response.headers
```

## Complete Example

```python
import http
import json

# GET request with error handling
try:
    response = http.get("https://jsonplaceholder.typicode.com/posts/1")
    response.raise_for_status()

    post = response.json()
    print("Post title:", post["title"])
    print("Status code:", response.status_code)

except Exception as e:
    print("Error:", e)

# POST request
try:
    new_post = {
        "title": "My Post",
        "body": "This is my post content",
        "userId": 1
    }

    body = json.stringify(new_post)
    options = {"timeout": 15}

    response = http.post("https://jsonplaceholder.typicode.com/posts", body, options)
    response.raise_for_status()

    created = response.json()
    print("Created post ID:", created["id"])

except Exception as e:
    print("Error:", e)
```

## Notes

- HTTP/2 support with automatic fallback to HTTP/1.1
- Connection pooling (100 connections per host)
- Accepts self-signed certificates
- Default timeout: 5 seconds
- Python requests-compatible API