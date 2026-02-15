import scriptling.runtime as runtime

# Register HTTP routes
runtime.http.get("/status", "handlers.status")
runtime.http.get("/counter", "handlers.get_counter")

# Register a background task that increments a counter every second
runtime.background("counter_task", "tasks.increment_counter")

print("Server configured with routes and background tasks")
