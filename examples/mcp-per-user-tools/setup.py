import scriptling.runtime as runtime

# Every request — HTTP routes, /mcp, /json-rpc and WebSocket upgrades alike —
# runs the middleware first. It authenticates the caller and registers the
# MCP entries they are allowed to see for the life of that request.
runtime.http.middleware("auth.check")
