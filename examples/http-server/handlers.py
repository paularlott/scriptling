import scriptling.runtime as runtime

# Simple in-memory user store
_users = [
    {"id": 1, "name": "Alice"},
    {"id": 2, "name": "Bob"},
]
_next_id = 3

def auth_middleware(request):
    """Authentication middleware - blocks requests without valid token."""
    # Skip auth for health endpoint
    if request.path == "/health":
        return None

    token = request.headers.get("authorization", "")

    if not token.startswith("Bearer "):
        return runtime.http.json(401, {"error": "Missing authorization token"})

    # Simple token validation (in production, use proper auth)
    if token != "Bearer secret123":
        return runtime.http.json(403, {"error": "Invalid token"})

    # Return None to continue to the handler
    return None


def list_users(request):
    """List all users."""
    return runtime.http.json(200, {"users": _users})


def create_user(request):
    """Create a new user from JSON body."""
    global _next_id

    data = request.json()
    if not data or "name" not in data:
        return runtime.http.json(400, {"error": "Missing 'name' field"})

    user = {"id": _next_id, "name": data["name"]}
    _users.append(user)
    _next_id += 1

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
