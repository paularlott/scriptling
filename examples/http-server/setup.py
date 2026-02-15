import scriptling.http

# Register middleware for authentication
scriptling.http.middleware("handlers.auth_middleware")

# Protected API routes
scriptling.http.get("/api/users", "handlers.list_users")
scriptling.http.post("/api/users", "handlers.create_user")
scriptling.http.get("/api/search", "handlers.search")
