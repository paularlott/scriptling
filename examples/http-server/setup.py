import scriptling.runtime as runtime

# Register middleware for authentication
runtime.http.middleware("handlers.auth_middleware")

# Protected API routes
runtime.http.get("/api/users", "handlers.list_users")
runtime.http.post("/api/users", "handlers.create_user")
runtime.http.get("/api/search", "handlers.search")
