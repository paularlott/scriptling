# Runtime Sync Library

Named cross-environment concurrency primitives shared across all environments.

## Setup

```go
import "github.com/paularlott/scriptling/extlibs"

// Register the Sync sub-library
extlibs.RegisterRuntimeSyncLibrary(p)
```

## Functions

### scriptling.runtime.sync.Atomic(name, initial=0)

Get or create a named atomic counter.

**Parameters:**

- `name` (string): Unique name for the counter
- `initial` (int, optional): Initial value (only used if creating new)

**Returns:** Atomic object with methods:

- `add(delta=1)`: Atomically add delta and return new value
- `get()`: Atomically read the value
- `set(value)`: Atomically set the value

### scriptling.runtime.sync.Shared(name, initial)

Get or create a named shared variable.

**Parameters:**

- `name` (string): Unique name for the variable
- `initial`: Initial value (only used if creating new)

**Returns:** Shared object with methods:

- `get()`: Get the current value (thread-safe)
- `set(value)`: Set the value (thread-safe)

### scriptling.runtime.sync.WaitGroup(name)

Get or create a named wait group.

**Parameters:**

- `name` (string): Unique name for the wait group

**Returns:** WaitGroup object with methods:

- `add(delta=1)`: Add to the counter
- `done()`: Decrement the counter
- `wait()`: Block until counter reaches zero

### scriptling.runtime.sync.Queue(name, maxsize=0)

Get or create a named thread-safe queue.

**Parameters:**

- `name` (string): Unique name for the queue
- `maxsize` (int, optional): Maximum queue size (0 = unbounded)

**Returns:** Queue object with methods:

- `put(item)`: Add item to queue (blocks if full)
- `get()`: Remove and return item (blocks if empty)
- `size()`: Return number of items in queue
- `close()`: Close the queue

## Examples

### Atomic Counter

```python
import scriptling.runtime as runtime

counter = runtime.sync.Atomic("requests", 0)
counter.add(1)      # Atomic increment
counter.add(-5)     # Atomic add
counter.set(100)    # Atomic set
value = counter.get()  # Atomic read
```

### Shared Variable

```python
import scriptling.runtime as runtime

shared = runtime.sync.Shared("config", {"debug": True})

# Read
config = shared.get()

# Write
config["debug"] = False
shared.set(config)
```

### WaitGroup

```python
import scriptling.runtime as runtime

wg = runtime.sync.WaitGroup("tasks")

def worker(id):
    print(f"Worker {id}")
    wg.done()

for i in range(10):
    wg.add(1)
    runtime.run(worker, i)

wg.wait()  # Wait for all workers
```

### Queue

```python
import scriptling.runtime as runtime

queue = runtime.sync.Queue("jobs", maxsize=100)

# Producer
def producer():
    for i in range(10):
        queue.put(i)

# Consumer
def consumer():
    while True:
        item = queue.get()
        if item is None:
            break
        process(item)

runtime.run(producer)
runtime.run(consumer)
```

### Cross-Environment Sync

```python
import scriptling.runtime as runtime

# In HTTP handler
def handler(request):
    queue = runtime.sync.Queue("jobs")
    queue.put({"task": "process", "data": request.json()})
    return runtime.http.json(200, {"status": "queued"})

# In background task
def worker():
    queue = runtime.sync.Queue("jobs")
    while True:
        job = queue.get()
        process_job(job)
```

## Notes

- All primitives are named and shared across environments
- Names must be unique within each primitive type
- Atomic operations are lock-free
- Queue supports bounded and unbounded modes
- WaitGroup follows Go's sync.WaitGroup semantics
