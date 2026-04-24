import scriptling.runtime as runtime

# Register middleware for authentication
runtime.http.middleware("handlers.auth_middleware")

# Protected API routes
runtime.http.get("/api/users", "handlers.list_users")
runtime.http.get("/api/users/me", "handlers.get_me")
runtime.http.post("/api/users", "handlers.create_user")
runtime.http.get("/api/search", "handlers.search")

# Custom 404 handler (used when no route matches and asset not found)
runtime.http.not_found("handlers.not_found")
