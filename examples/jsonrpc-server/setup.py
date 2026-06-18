import scriptling.runtime as runtime

# Methods: each maps a JSON-RPC method name to a "library.function" handler.
runtime.jsonrpc.method("echo", "handlers.echo")
runtime.jsonrpc.method("add", "handlers.add")
runtime.jsonrpc.method("divide", "handlers.divide")

# Notifications: requests without an id; no response is written.
runtime.jsonrpc.notification("progress", "handlers.on_progress")
