# scriptling.http

HTTP server route registration library for building web applications and webhooks.

## Available Functions

| Function                | Description                       |
| ----------------------- | --------------------------------- |
| `get(path, handler)`    | Register a GET route              |
| `post(path, handler)`   | Register a POST route             |
| `put(path, handler)`    | Register a PUT route              |
| `delete(path, handler)` | Register a DELETE route           |
| `static(path, dir)`     | Serve static files from directory |
| `middleware(handler)`   | Add middleware for all routes     |

## Overview

The `scriptling.http` library provides functions to register HTTP routes that are served by the Scriptling HTTP server. Use this library in your setup script to define how your application responds to HTTP requests.

**Key features:**

- Register handlers for GET, POST, PUT, DELETE methods
- Middleware support for authentication, logging, etc.
- Static file serving
- JSON, HTML, and text response helpers
- Request parsing utilities

## Server Mode

To use HTTP routes, start Scriptling in server mode:

```bash
# Start server with setup script
scriptling --server :8000 setup.py

# With TLS
scriptling --server :8443 --tls-generate setup.py

# With MCP tools and custom routes
scriptling --server :8000 --mcp-tools ./tools setup.py
```

## Functions

### scriptling.http.get(path, handler)

Register a GET route.

**Parameters:**

- `path` (str): URL path for the route (e.g., "/api/users")
- `handler` (str): Handler function as "library.function" string

**Example:**

```python
import scriptling.http

scriptling.http.get("/health", "handlers.health_check")
scriptling.http.get("/api/users", "handlers.list_users")
```

### scriptling.http.post(path, handler)

Register a POST route.

**Parameters:**

- `path` (str): URL path for the route
- `handler` (str): Handler function as "library.function" string

**Example:**

```python
import scriptling.http

scriptling.http.post("/api/users", "handlers.create_user")
scriptling.http.post("/webhook", "handlers.webhook")
```

### scriptling.http.put(path, handler)

Register a PUT route.

**Parameters:**

- `path` (str): URL path for the route
- `handler` (str): Handler function as "library.function" string

**Example:**

```python
import scriptling.http

scriptling.http.put("/api/users/:id", "handlers.update_user")
```

### scriptling.http.delete(path, handler)

Register a DELETE route.

**Parameters:**

- `path` (str): URL path for the route
- `handler` (str): Handler function as "library.function" string

**Example:**

```python
import scriptling.http

scriptling.http.delete("/api/users/:id", "handlers.delete_user")
```

### scriptling.http.route(path, handler, methods=["GET", "POST", "PUT", "DELETE"])

Register a route for multiple HTTP methods.

**Parameters:**

- `path` (str): URL path for the route
- `handler` (str): Handler function as "library.function" string
- `methods` (list): List of HTTP methods to accept

**Example:**

```python
import scriptling.http

# Accept GET and POST only
scriptling.http.route("/api/data", "handlers.data", methods=["GET", "POST"])

# Accept all common methods
scriptling.http.route("/api/resource", "handlers.resource")
```

### scriptling.http.middleware(handler)

Register middleware for all routes.

**Parameters:**

- `handler` (str): Middleware function as "library.function" string

The middleware receives the request object and should return:

- None to continue to the handler
- A response dict to short-circuit (block the request)

**Example:**

```python
import scriptling.http

scriptling.http.middleware("auth.check_request")
```

### scriptling.http.static(path, directory)

Register a static file serving route.

**Parameters:**

- `path` (str): URL path prefix for static files (e.g., "/assets")
- `directory` (str): Local directory to serve files from

**Example:**

```python
import scriptling.http

scriptling.http.static("/assets", "./public")
scriptling.http.static("/static", "/var/www/static")
```

### scriptling.http.json(status_code, data)

Create a JSON response.

**Parameters:**

- `status_code` (int): HTTP status code (e.g., 200, 404, 500)
- `data`: Data to serialize as JSON

**Returns:** dict - Response object for the server

**Example:**

```python
import scriptling.http

# Success response
return scriptling.http.json(200, {"status": "ok", "data": result})

# Error response
return scriptling.http.json(404, {"error": "User not found"})

# Or with status kwarg
return scriptling.http.json({"items": []}, status=200)
```

### scriptling.http.redirect(location, status=302)

Create a redirect response.

**Parameters:**

- `location` (str): URL to redirect to
- `status` (int, optional): HTTP status code (default: 302)

**Returns:** dict - Response object for the server

**Example:**

```python
import scriptling.http

# Temporary redirect
return scriptling.http.redirect("/new-location")

# Permanent redirect
return scriptling.http.redirect("/permanent", status=301)
```

### scriptling.http.html(status_code, content)

Create an HTML response.

**Parameters:**

- `status_code` (int): HTTP status code
- `content` (str): HTML content to return

**Returns:** dict - Response object for the server

**Example:**

```python
import scriptling.http

return scriptling.http.html(200, "<h1>Hello World</h1>")
return scriptling.http.html("<p>Simple content</p>", status=200)
```

### scriptling.http.text(status_code, content)

Create a plain text response.

**Parameters:**

- `status_code` (int): HTTP status code
- `content` (str): Text content to return

**Returns:** dict - Response object for the server

**Example:**

```python
import scriptling.http

return scriptling.http.text(200, "Hello World")
return scriptling.http.text("Plain text", status=200)
```

### scriptling.http.parse_query(query_string)

Parse a URL query string.

**Parameters:**

- `query_string` (str): Query string to parse (with or without leading ?)

**Returns:** dict - Parsed key-value pairs

**Example:**

```python
import scriptling.http

params = scriptling.http.parse_query("name=John&age=30")
# {"name": "John", "age": "30"}

# Multiple values become lists
params = scriptling.http.parse_query("tag=a&tag=b")
# {"tag": ["a", "b"]}
```

## Request Object

Handlers receive a Request object with the following attributes:

| Attribute | Type | Description                      |
| --------- | ---- | -------------------------------- |
| `method`  | str  | HTTP method (GET, POST, etc.)    |
| `path`    | str  | URL path                         |
| `body`    | str  | Request body as string           |
| `headers` | dict | Request headers (lowercase keys) |
| `query`   | dict | Query parameters                 |

### Request.json()

Parse the request body as JSON.

**Returns:** dict or list or None

**Example:**

```python
def handle_post(request):
    data = request.json()
    name = data.get("name", "anonymous")
    return scriptling.http.json(200, {"greeting": f"Hello, {name}!"})
```

## Usage Examples

### Basic API

**setup.py:**

```python
import scriptling.http

scriptling.http.get("/api/health", "api.health")
scriptling.http.get("/api/users", "api.list_users")
scriptling.http.post("/api/users", "api.create_user")
```

**api.py:**

```python
import scriptling.http

def health(request):
    return scriptling.http.json(200, {"status": "healthy"})

def list_users(request):
    users = [{"id": 1, "name": "Alice"}, {"id": 2, "name": "Bob"}]
    return scriptling.http.json(200, {"users": users})

def create_user(request):
    data = request.json()
    # ... create user ...
    return scriptling.http.json(201, {"id": 3, "name": data["name"]})
```

### Authentication Middleware

**auth.py:**

```python
import scriptling.http

def check_request(request):
    token = request.headers.get("authorization", "")

    if not token.startswith("Bearer "):
        return scriptling.http.json(401, {"error": "Missing token"})

    # Validate token
    if token != "Bearer secret123":
        return scriptling.http.json(403, {"error": "Invalid token"})

    # Return None to continue to handler
    return None
```

**setup.py:**

```python
import scriptling.http

scriptling.http.middleware("auth.check_request")
scriptling.http.get("/api/protected", "handlers.protected")
```

### Telegram Webhook

**setup.py:**

```python
import scriptling.http
import scriptling.kv
import os

# Store bot token for handlers
scriptling.kv.set("telegram_token", os.environ["TELEGRAM_TOKEN"])

# Register webhook endpoint
scriptling.http.post("/telegram/webhook", "telegram.webhook")
```

**telegram.py:**

```python
import scriptling.http
import scriptling.kv

def webhook(request):
    token = scriptling.kv.get("telegram_token")
    bot = telegram.Bot(token)

    update = request.json()

    if "message" in update:
        chat_id = update["message"]["chat"]["id"]
        text = update["message"].get("text", "")

        bot.send_message(chat_id, f"Echo: {text}")

    return scriptling.http.json(200, {"status": "ok"})
```

## See Also

- [scriptling.kv](kv.md) - Thread-safe key-value store
- [requests](requests.md) - HTTP client library
