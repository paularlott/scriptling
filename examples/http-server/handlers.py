import scriptling.runtime as runtime

# Static user data for the example
_users = [
    {"id": 1, "name": "Alice"},
    {"id": 2, "name": "Bob"},
    {"id": 3, "name": "Charlie"},
]

def auth_middleware(request):
    """Authentication middleware - protects POST /api/users and GET /api/users/me."""
    protected = (
        (request.method == "POST" and request.path == "/api/users") or
        (request.method == "GET" and request.path == "/api/users/me")
    )
    if not protected:
        return None

    token = request.headers.get("authorization", "")
    if not token.startswith("Bearer "):
        return runtime.http.json(401, {"error": "Missing authorization token"})
    if token != "Bearer secret123":
        return runtime.http.json(403, {"error": "Invalid token"})

    return None


def list_users(request):
    """List all users."""
    return runtime.http.json(200, {"users": _users})


def create_user(request):
    """Create a new user from JSON body (example only - not persisted)."""
    data = request.json()
    if not data or "name" not in data:
        return runtime.http.json(400, {"error": "Missing 'name' field"})

    user = {"id": len(_users) + 1, "name": data["name"]}
    return runtime.http.json(201, {"user": user})


def search(request):
    """Search with query parameters."""
    query = request.query.get("q", "")
    limit = int(request.query.get("limit", "10"))

    results = []
    for user in _users:
        if query.lower() in user["name"].lower():
            results.append(user)
            if len(results) >= limit:
                break

    return runtime.http.json(200, {"query": query, "results": results})


def get_me(request):
    """Return the current user profile (requires auth)."""
    return runtime.http.json(200, {"id": 1, "name": "Alice", "email": "alice@example.com", "role": "admin"})


def not_found(request):
    """Custom 404 handler."""
    body = """<!DOCTYPE html>
<html>
<head><title>404 Not Found</title><link rel="stylesheet" href="/style.css"></head>
<body>
  <h1>404 - Page Not Found</h1>
  <p>The page could not be found.</p>
  <p><a href="/">Go home</a></p>
</body>
</html>"""
    return runtime.http.html(404, body)
