# HTTP Server Example

This example demonstrates how to build an HTTP server using the `scriptling.runtime.http` library.

## What It Shows

- Registering GET and POST routes
- Path parameters in route patterns (`/api/users/{id}`) read via `request.path_param()`
- Handler modules in subdirectories (`routes/status.py` imported as `import routes.status`)
- Using handler libraries to separate route setup from request handling
- Returning JSON responses
- Accessing request data (headers, query params, body)
- Authentication middleware
- Custom 404 handler via `runtime.http.not_found()`
- Serving static files with `--web-root`

## Files

| File | Purpose |
|------|---------|
| `setup.py` | Entry point - registers routes, middleware, and 404 handler |
| `handlers.py` | Request handler functions |
| `routes/status.py` | Handler module in a subdirectory (referenced as `routes.status.*`) |
| `assets/` | Static files served when no route matches |

## Running the Example

Start the server from the project root, with the web root directory:

```bash
scriptling --server :8000 --web-root examples/http-server/assets examples/http-server/setup.py
```

Or with TLS:

```bash
scriptling --server :8443 --tls-generate --web-root examples/http-server/assets examples/http-server/setup.py
```

## Testing the Endpoints

Once the server is running:

```bash
# Static files (served from the web root directory)
curl http://localhost:8000/
curl http://localhost:8000/about.html
curl http://localhost:8000/style.css

# Health check (built-in route)
curl http://localhost:8000/health

# API - list users (public)
curl http://localhost:8000/api/users

# API - fetch one user by path parameter (public)
curl http://localhost:8000/api/users/2

# API - search (public)
curl "http://localhost:8000/api/search?q=alice"

# API - component status from the subdirectory module (public)
curl http://localhost:8000/api/status/api

# API - create a user (requires auth)
curl -X POST http://localhost:8000/api/users \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer secret123" \
  -d '{"name": "Charlie"}'

# API - search
curl "http://localhost:8000/api/search?q=alice"

# Custom 404 handler (no route and no matching file in web root)
curl http://localhost:8000/missing.html
```

## Key Points

- `--web-root <dir>` serves files from the directory when no route matches the URL
- Route patterns support `{name}` for one path segment and `{name...}` for the rest of the path; values are read with `request.path_param("name")` and arrive percent-decoded
- A literal route wins over a wildcard at the same position, so `/api/users/me` still hits its own handler while `/api/users/2` matches `/api/users/{id}`
- `runtime.http.not_found("handlers.not_found")` registers a custom 404 handler
- The 404 handler is called when no route matches **and** no file is found in the web root
- If no 404 handler is registered, the server returns a plain `404 Not Found`
- `/health` is a built-in route that's always available
- Middleware can skip auth for non-API paths (e.g., static files)

## See Also

- [scriptling.runtime.http documentation](https://scriptling.dev/docs/libraries/scriptling/runtime-http/)
