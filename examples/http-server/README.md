# HTTP Server Example

This example demonstrates how to build an HTTP server using the `scriptling.runtime.http` library.

## What It Shows

- Registering GET and POST routes
- Using handler libraries to separate route setup from request handling
- Returning JSON responses
- Accessing request data (headers, query params, body)
- Authentication middleware

## Files

| File | Purpose |
|------|---------|
| `setup.py` | Entry point - registers routes and middleware |
| `handlers.py` | Request handler functions |

## Running the Example

Start the server from the project root:

```bash
scriptling --server :8000 examples/http-server/setup.py
```

Or with TLS:

```bash
scriptling --server :8443 --tls-generate examples/http-server/setup.py
```

## Testing the Endpoints

Once the server is running, test the endpoints:

```bash
# Health check (built-in route, always available)
curl http://localhost:8000/health

# Get users list
curl http://localhost:8000/api/users

# Create a user (requires auth token)
curl -X POST http://localhost:8000/api/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer secret123" \
  -d '{"name": "Alice"}'

# Query parameters
curl "http://localhost:8000/api/search?q=test&limit=10"

# Unauthorized request (will be blocked by middleware)
curl -X POST http://localhost:8000/api/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Bob"}'
```

## Key Points

- `/health` is a built-in route that's always available - no need to register it
- Routes are registered using `scriptling.runtime.http.get()`, `scriptling.runtime.http.post()`, etc.
- Handlers are specified as `"library.function"` strings (e.g., `"handlers.health"`)
- Middleware can intercept requests and return early responses
- Use `scriptling.runtime.http.json()` to return JSON responses
- Request objects provide `method`, `path`, `headers`, `query`, and `body`

## See Also

- [scriptling.runtime.http documentation](https://scriptling.dev/docs/libraries/scriptling/runtime-http/)
- [scriptling.runtime.kv documentation](https://scriptling.dev/docs/libraries/scriptling/runtime-kv/) - Thread-safe key-value store
