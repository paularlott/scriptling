# Runtime Background Tasks Example

Demonstrates the `scriptling.runtime` library features including background tasks, concurrent execution, synchronization primitives, and key-value storage.

## Features Demonstrated

### Background Task Execution
```python
runtime.background("task_name", "function_name", *args, **kwargs)
```
Register and start background tasks with arguments. Returns a Promise to get results.

- **Script Mode**: Tasks start immediately when `background()` is called
- **HTTP Server Mode**: Tasks are queued during setup, then started when server is ready

### 2. Synchronization Primitives

**WaitGroup** - Coordinate multiple goroutines:
```python
wg = runtime.sync.WaitGroup("name")
wg.add(3)
# ... start tasks ...
wg.wait()
```

**Atomic** - Thread-safe counter:
```python
counter = runtime.sync.Atomic("name", 0)
count = counter.add(1)
value = counter.get()
```

**Shared** - Thread-safe shared state:
```python
shared = runtime.sync.Shared("name", {"count": 0})
data = shared.get()
shared.set(updated_data)
```

**Queue** - Producer-consumer pattern:
```python
queue = runtime.sync.Queue("name", maxsize=10)
queue.put(item)
item = queue.get()
```

### 3. Key-Value Store
```python
kv = runtime.kv
kv.set("key", "value", ttl=60)  # Optional TTL in seconds
value = kv.get("key")
kv.incr("counter", 1)
kv.exists("key")
kv.keys()
```

## Running the Example

```bash
# Build the CLI first (from repo root)
task build

# Run the example
./bin/scriptling examples/background/example.py
```

## Example Output

The example shows:
1. Three background tasks starting and logging their progress
2. Tasks coordinated with WaitGroup synchronization
3. Shared state tracking task status
4. Concurrent calculations with runtime.run()
5. KV store operations with TTL support

All tasks run concurrently and their output is interleaved, demonstrating true concurrent execution.

## Use Cases

- **HTTP Server Background Tasks**: Process webhooks, cleanup jobs, monitoring
- **Concurrent Processing**: Parallel calculations, batch operations
- **State Management**: Shared counters, caches, session data
- **Task Coordination**: Wait for multiple operations, producer-consumer patterns
