# Runtime Library

Background tasks for HTTP servers.

## Available Functions

| Function                    | Description                |
| --------------------------- | -------------------------- |
| `background(name, handler)` | Register a background task |

## Setup

```go
import "github.com/paularlott/scriptling/extlibs"

// Register only the core runtime library (background function)
extlibs.RegisterRuntimeLibrary(p)

// Optionally register sub-libraries individually
extlibs.RegisterRuntimeHTTPLibrary(p)   // HTTP routes and responses
extlibs.RegisterRuntimeKVLibrary(p)     // Key-value store
extlibs.RegisterRuntimeSyncLibrary(p)   // Concurrency primitives

// Or register everything at once (core + all sub-libraries)
extlibs.RegisterRuntimeLibraryAll(p)
```

## Functions

### scriptling.runtime.background(name, handler)

Register a background task.

**Parameters:**

- `name` (string): Unique name for the task (used in logs and error messages)
- `handler` (string): Handler function as "library.function"
- `*args`: Positional arguments to pass to the function
- `**kwargs`: Keyword arguments to pass to the function

Background tasks run in separate goroutines alongside the HTTP server. The name is used to identify the task in server logs.

### scriptling.runtime.run(func, \*args, \*\*kwargs)

Run function asynchronously within the current environment.

**Parameters:**

- `func`: Function to execute
- `*args`: Positional arguments
- `**kwargs`: Keyword arguments

**Returns:** Promise object with methods:

- `get()`: Wait for and return the result
- `wait()`: Wait for completion and discard the result

## Sub-Libraries

- [scriptling.runtime.http](runtime-http.md) - HTTP route registration and response helpers
- [scriptling.runtime.kv](runtime-kv.md) - Thread-safe key-value store
- [scriptling.runtime.sync](runtime-sync.md) - Named cross-environment concurrency primitives
- [scriptling.runtime.sandbox](runtime-sandbox.md) - Isolated script execution environments

## Examples

### Background Task

```python
# setup.py
import scriptling.runtime as runtime

runtime.http.get("/counter", "handlers.get_counter")
runtime.background("counter_task", "tasks.increment_counter")
```

```python
# tasks.py
import scriptling.runtime as runtime
import time

def increment_counter():
    counter = runtime.sync.Atomic("request_counter", 0)
    while True:
        counter.add(1)
        print(f"Counter: {counter.get()}")
        time.sleep(1)
```

## Notes

- Background tasks run in isolated environments
- Use named sync primitives to share state between tasks and handlers
- Background tasks start automatically when the server starts
