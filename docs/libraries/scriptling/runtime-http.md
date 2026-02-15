# Runtime HTTP Library

HTTP server route registration and response helpers.

## Setup

```go
import "github.com/paularlott/scriptling/extlibs"

// Register the HTTP sub-library
extlibs.RegisterRuntimeHTTPLibrary(p)
```

## Route Registration

### scriptling.runtime.http.get(path, handler)

Register a GET route.

**Parameters:**
- `path` (string): URL path (e.g., "/api/users")
- `handler` (string): Handler function as "library.function"

### scriptling.runtime.http.post(path, handler)

Register a POST route.

### scriptling.runtime.http.put(path, handler)

Register a PUT route.

### scriptling.runtime.http.delete(path, handler)

Register a DELETE route.

### scriptling.runtime.http.route(path, handler, methods=[])

Register a route for multiple HTTP methods.

**Parameters:**
- `path` (string): URL path
- `handler` (string): Handler function
- `methods` (list, optional): HTTP methods (default: all)

### scriptling.runtime.http.middleware(handler)

Register global middleware.

**Parameters:**
- `handler` (string): Middleware function

The middleware receives the request and should return:
- `None` to continue to the handler
- A response dict to short-circuit

### scriptling.runtime.http.static(path, directory)

Register static file serving.

**Parameters:**
- `path` (string): URL path prefix (e.g., "/assets")
- `directory` (string): Local directory to serve

## Response Helpers

### scriptling.runtime.http.json(status_code, data)

Create a JSON response.

**Parameters:**
- `status_code` (int): HTTP status code
- `data`: Data to serialize as JSON

**Returns:** Response dict

### scriptling.runtime.http.html(status_code, content)

Create an HTML response.

**Parameters:**
- `status_code` (int): HTTP status code
- `content` (string): HTML content

**Returns:** Response dict

### scriptling.runtime.http.text(status_code, content)

Create a plain text response.

**Parameters:**
- `status_code` (int): HTTP status code
- `content` (string): Text content

**Returns:** Response dict

### scriptling.runtime.http.redirect(location, status=302)

Create a redirect response.

**Parameters:**
- `location` (string): URL to redirect to
- `status` (int, optional): HTTP status code (default: 302)

**Returns:** Response dict

### scriptling.runtime.http.parse_query(query_string)

Parse a URL query string.

**Parameters:**
- `query_string` (string): Query string to parse

**Returns:** Dict of key-value pairs

## Request Object

Handlers receive a Request object with these fields:

- `method` (string): HTTP method
- `path` (string): Request path
- `body` (string): Request body
- `headers` (dict): Request headers (lowercase keys)
- `query` (dict): Query parameters

**Methods:**
- `json()`: Parse body as JSON

## Examples

### Basic Routes

```python
# setup.py
import scriptling.runtime as runtime

runtime.http.get("/users", "handlers.list_users")
runtime.http.post("/users", "handlers.create_user")
runtime.http.middleware("handlers.auth")
```

### Handler with Response

```python
# handlers.py
import scriptling.runtime as runtime

def list_users(request):
    users = runtime.kv.get("users", default=[])
    return runtime.http.json(200, {"users": users})

def create_user(request):
    data = request.json()
    users = runtime.kv.get("users", default=[])
    users.append(data)
    runtime.kv.set("users", users)
    return runtime.http.json(201, {"user": data})
```

### Middleware

```python
import scriptling.runtime as runtime

def auth(request):
    if "authorization" not in request.headers:
        return runtime.http.json(401, {"error": "Unauthorized"})
    return None  # Continue to handler
```

## Notes

- Routes are registered during setup script execution
- Response helpers return dicts compatible with the server
- Middleware can short-circuit requests by returning a response
