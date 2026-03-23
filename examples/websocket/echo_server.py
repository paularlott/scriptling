# WebSocket Echo Server Example
# Run with: scriptling-cli serve --script examples/websocket/echo_server.py

import scriptling.runtime as runtime

# Register WebSocket endpoint
runtime.http.websocket("/echo", "handlers.echo_handler")

# Also register a simple HTTP endpoint for testing
runtime.http.get("/", "handlers.home")
