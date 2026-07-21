import scriptling.runtime as runtime

runtime.jsonrpc.method("echo", "handlers.echo")
runtime.jsonrpc.method("add", "handlers.add")
runtime.jsonrpc.method("divide", "handlers.divide")

runtime.jsonrpc.notification("progress", "handlers.on_progress")
