# threads - Asynchronous Execution Library

Go-inspired async library for safe concurrent execution through isolated environments.

## Available Functions

| Function                     | Description                                  |
| ---------------------------- | -------------------------------------------- |
| `run(func, *args, **kwargs)` | Run function asynchronously in goroutine     |
| `Atomic(value)`              | Create atomic value for thread-safe ops      |
| `Shared(value)`              | Create shared value with mutex               |
| `Queue(maxsize=0)`           | Create thread-safe queue                     |
| `WaitGroup()`                | Create wait group for goroutine coordination |

## Design Principles

- **Isolated Environments** - Each goroutine gets an empty environment with only libraries
- **Explicit Data Passing** - All data must be passed as parameters (no implicit sharing)
- **Context-Based Cleanup** - Goroutines cancelled when script context is cancelled
- **Promise-Based** - Returns Promise objects (familiar from JavaScript)
- **Go-Inspired Primitives** - WaitGroup, Queue, Pool, Atomic, Shared operations

## Important: Thread Safety

Thread functions **do not** have access to variables from the parent scope.
All data must be passed explicitly as parameters.

For shared mutable data, use thread-safe primitives: `Atomic()`, `Shared()`, `Queue()`.

## Functions

### threads.run(func, \*args, \*\*kwargs)

Run function asynchronously in a separate goroutine with isolated environment.

**Parameters:**

- `func` - Function to execute
- `*args` - Positional arguments to pass to the function
- `**kwargs` - Keyword arguments to pass to the function

**Returns:** Promise object with `.get()` and `.wait()` methods

**Note:** The spawned thread has access to libraries but NOT to parent scope variables.

```python
import scriptling.threads as threads

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

#### Thread Safety with Shared Data

Use `Atomic()` for counters and `Shared()` for complex values:

```python
import scriptling.threads as threads

counter = threads.Atomic(0)

def increment(counter):
    counter.add(1)  # Thread-safe

promises = [threads.run(increment, counter) for _ in range(10)]
for p in promises:
    p.get()

print(counter.get())  # 10
```

#### Passing Data

All data must be passed explicitly:

```python
import scriptling.threads as threads

def process_data(data):
    return [x * 2 for x in data]

my_data = [1, 2, 3, 4, 5]
promise = threads.run(process_data, my_data)
result = promise.get()  # [2, 4, 6, 8, 10]
```

### Promise.wait()

Wait for async operation to complete and discard the result.

**Returns:** null (when operation completes)

```python
import scriptling.threads as threads

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
import scriptling.threads as threads

counter = threads.Atomic(0)

def increment(counter):
    counter.add(1)

promises = [threads.run(increment, counter) for _ in range(1000)]
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
import scriptling.threads as threads

shared_list = threads.Shared([])

def append_item(shared_list, item):
    current = shared_list.get()
    current.append(item)
    shared_list.set(current)

promises = [threads.run(append_item, shared_list, i) for i in range(100)]
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
import scriptling.threads as threads

wg = threads.WaitGroup()

def worker(wg, id):
    print(f"Worker {id} starting")
    # ... do work ...
    print(f"Worker {id} done")
    wg.done()

for i in range(10):
    wg.add(1)
    threads.run(worker, wg, i)

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
import scriptling.threads as threads

queue = threads.Queue(maxsize=100)

def producer(queue):
    for i in range(10):
        queue.put(i)
    queue.put(None)  # Sentinel

def consumer(queue):
    while True:
        item = queue.get()
        if item is None:
            break
        print(f"Processing {item}")

threads.run(producer, queue)
threads.run(consumer, queue)
```

### threads.Pool(worker_func, workers=4, queue_depth=workers\*2)

Create a worker pool for processing data items.

**Parameters:**

- `worker_func` - Function to process each item
- `workers` - Number of worker goroutines
- `queue_depth` - Maximum queued items

**Methods:**

- `submit(data)` - Submit data to pool for processing
- `close()` - Stop accepting work and wait for completion

```python
import scriptling.threads as threads

def process_data(item):
    result = item * item
    print(f"Processed {item} -> {result}")
    return result

pool = threads.Pool(process_data, workers=4, queue_depth=1000)

for item in range(100):
    pool.submit(item)

pool.close()  # Wait for all work to complete
```

## Thread Safety Model

**Isolation by default:**

- Each goroutine gets an empty environment with only libraries
- No access to parent scope variables
- All data must be passed explicitly as parameters

**Sharing through primitives:**

- `Atomic()` - Lock-free atomic integers
- `Shared()` - Mutex-protected shared values
- `Queue()` - Thread-safe communication channel
- `WaitGroup()` - Synchronization primitive

**No implicit sharing prevents:**

- Data races from accidental shared state
- Memory overhead from deep copying
- Performance degradation from expensive clones

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

1. **Pass data explicitly** - All data must be passed as function parameters
2. **Use Atomic for counters** - Lock-free and fast
3. **Use Shared for complex types** - When you need mutex protection
4. **Use WaitGroup for synchronization** - Wait for multiple operations
5. **Use Queue for producer-consumer** - Thread-safe communication
6. **Use Pool for batch processing** - Efficient worker management
7. **Use promise.wait() for fire-and-forget** - When you don't need the result
8. **Use promise.get() when needed** - Wait and return the computed value

## Performance Notes

- **O(1) environment creation** - No deep copy overhead
- **Atomic operations are lock-free** and very fast
- **Pool reuses goroutines** for efficiency
- **Queue uses condition variables** for blocking
- **Explicit data passing** is faster than cloning large environments
