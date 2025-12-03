# threads - Asynchronous Execution Library

Go-inspired async library for safe concurrent execution through isolated environments.

## Design Principles

- **Isolated Environments** - Each goroutine gets a cloned environment (safe by default)
- **Context-Based Cleanup** - Goroutines cancelled when script context is cancelled
- **Promise-Based** - Returns Promise objects (familiar from JavaScript)
- **Go-Inspired Primitives** - WaitGroup, Queue, Pool, Atomic operations

## Functions

### threads.run(func, *args, **kwargs)

Run function asynchronously in a separate goroutine with isolated environment.

**Returns:** Promise object with `.get()` and `.wait()` methods

```python
import threads

def worker(x, y=10):
    return x + y

# With positional and keyword args
promise = threads.run(worker, 5, y=3)
result = promise.get()  # Returns 8

# With only keyword args
promise2 = threads.run(worker, x=7, y=3)
result2 = promise2.get()  # Returns 10

# Multiple async operations
promises = [threads.run(worker, i, y=i+1) for i in range(10)]
results = [p.get() for p in promises]
```

### Promise.wait()

Wait for async operation to complete and discard the result.

**Returns:** null (when operation completes)

```python
import threads

def worker(x, y=10):
    print(f"Processing {x} + {y} = {x + y}")

# Run async and wait for completion (fire-and-forget style)
promise = threads.run(worker, 5, y=3)
promise.wait()  # Waits for completion, discards result
# Function completes before promise.wait() returns
```

### threads.Atomic(initial=0)

Create an atomic integer counter for lock-free operations.

**Methods:**
- `add(delta=1)` - Atomically add delta and return new value
- `get()` - Atomically read the value
- `set(value)` - Atomically set the value

```python
import threads

counter = threads.Atomic(0)

def increment():
    counter.add(1)

promises = [threads.run(increment) for _ in range(1000)]
for p in promises:
    p.get()

print(counter.get())  # 1000
```

### threads.Shared(initial_value)

Create a thread-safe shared variable with mutex protection.

**Methods:**
- `get()` - Get the current value (thread-safe)
- `set(value)` - Set the value (thread-safe)

```python
import threads

shared_list = threads.Shared([])

def append_item(item):
    current = shared_list.get()
    current.append(item)
    shared_list.set(current)

promises = [threads.run(append_item, i) for i in range(100)]
for p in promises:
    p.get()

print(len(shared_list.get()))  # 100
```

### threads.WaitGroup()

Create a wait group for synchronizing goroutines (Go-style).

**Methods:**
- `add(delta=1)` - Add to the wait group counter
- `done()` - Decrement the wait group counter
- `wait()` - Block until counter reaches zero

```python
import threads

wg = threads.WaitGroup()

def worker(id):
    print(f"Worker {id} starting")
    # ... do work ...
    print(f"Worker {id} done")
    wg.done()

for i in range(10):
    wg.add(1)
    threads.run(worker, i)

wg.wait()
print("All workers complete")
```

### threads.Queue(maxsize=0)

Create a thread-safe queue for producer-consumer patterns.

**Parameters:**
- `maxsize` - Maximum queue size (0 = unbounded)

**Methods:**
- `put(item)` - Add item to queue (blocks if full)
- `get()` - Remove and return item from queue (blocks if empty)
- `size()` - Return number of items in queue
- `close()` - Close the queue

```python
import threads

queue = threads.Queue(maxsize=100)

def producer():
    for i in range(10):
        queue.put(i)
    queue.put(None)  # Sentinel

def consumer():
    while True:
        item = queue.get()
        if item is None:
            break
        print(f"Processing {item}")

threads.run(producer)
threads.run(consumer)
```

### threads.Pool(worker_func, workers=4, queue_depth=workers*2)

Create a worker pool for processing data items.

**Parameters:**
- `worker_func` - Function to process each item
- `workers` - Number of worker goroutines
- `queue_depth` - Maximum queued items

**Methods:**
- `submit(data)` - Submit data to pool for processing
- `close()` - Stop accepting work and wait for completion

```python
import threads

def process_data(item):
    result = item * item
    print(f"Processed {item} -> {result}")
    return result

pool = threads.Pool(process_data, workers=4, queue_depth=1000)

for item in range(100):
    pool.submit(item)

pool.close()  # Wait for all work to complete
```

## Thread Safety

All async operations use isolated environments created via deep copy:
- Each goroutine gets its own copy of variables
- Changes in one goroutine don't affect others
- Use `Shared()` or `Atomic()` for intentional sharing

## Context Cancellation

All async operations respect context cancellation:
- When script context is cancelled, all goroutines are stopped
- Use with `EvalWithTimeout()` for automatic cleanup

```python
# In Go code:
result, err := p.EvalWithTimeout(30*time.Second, script)
// All async operations will be cancelled after 30 seconds
```

## Best Practices

1. **Use promise.wait() for fire-and-forget operations** - When you don't need the result
2. **Use promise.get() when you need the result** - Wait and return the computed value
3. **Use Atomic for counters** - Lock-free and fast
4. **Use Shared for complex types** - When you need mutex protection
5. **Use WaitGroup for synchronization** - Wait for multiple operations
6. **Use Queue for producer-consumer** - Thread-safe communication
7. **Use Pool for batch processing** - Efficient worker management

## Performance Notes

- Environment cloning has overhead - use for I/O-bound tasks
- Atomic operations are lock-free and very fast
- Pool reuses goroutines for efficiency
- Queue uses condition variables for blocking
