# Runtime KV Library

Thread-safe key-value store for sharing state across requests and background tasks.

## Setup

```go
import "github.com/paularlott/scriptling/extlibs"

// Register the KV sub-library
extlibs.RegisterRuntimeKVLibrary(p)
```

## Functions

### scriptling.runtime.kv.set(key, value, ttl=0)

Store a value with optional TTL.

**Parameters:**

- `key` (string): Key to store under
- `value`: Value to store (string, int, float, bool, list, dict)
- `ttl` (int, optional): Time-to-live in seconds (0 = no expiration)

### scriptling.runtime.kv.get(key, default=None)

Retrieve a value by key.

**Parameters:**

- `key` (string): Key to retrieve
- `default`: Value to return if key doesn't exist

**Returns:** Stored value (deep copy) or default

### scriptling.runtime.kv.delete(key)

Remove a key from the store.

### scriptling.runtime.kv.exists(key)

Check if a key exists and is not expired.

**Returns:** Boolean

### scriptling.runtime.kv.incr(key, amount=1)

Atomically increment an integer value.

**Parameters:**

- `key` (string): Key to increment
- `amount` (int, optional): Amount to increment by (default: 1)

**Returns:** New value after incrementing

### scriptling.runtime.kv.ttl(key)

Get remaining time-to-live for a key.

**Returns:** Seconds remaining, -1 if no expiration, -2 if key doesn't exist

### scriptling.runtime.kv.keys(pattern="\*")

Get all keys matching a glob pattern.

**Parameters:**

- `pattern` (string, optional): Glob pattern (default: "\*")

**Returns:** List of matching keys

### scriptling.runtime.kv.clear()

Remove all keys from the store.

## Examples

### Basic Usage

```python
import scriptling.runtime as runtime

# Store values
runtime.kv.set("api_key", "secret123")
runtime.kv.set("session:abc", {"user": "bob"}, ttl=3600)

# Retrieve values
api_key = runtime.kv.get("api_key")
session = runtime.kv.get("session:abc", default={})

# Check existence
if runtime.kv.exists("session:abc"):
    print("Session exists")
```

### Counter

```python
import scriptling.runtime as runtime

# Initialize counter
runtime.kv.set("counter", 0)

# Increment atomically
runtime.kv.incr("counter")      # Returns 1
runtime.kv.incr("counter", 5)   # Returns 6
```

### Pattern Matching

```python
import scriptling.runtime as runtime

# Store multiple keys
runtime.kv.set("user:1", {"name": "Alice"})
runtime.kv.set("user:2", {"name": "Bob"})
runtime.kv.set("session:abc", {"user_id": 1})

# Get all user keys
user_keys = runtime.kv.keys("user:*")  # ["user:1", "user:2"]
```

### Sharing State

```python
import scriptling.runtime as runtime

# In HTTP handler
def handler(request):
    users = runtime.kv.get("users", default=[])
    users.append(request.json())
    runtime.kv.set("users", users)
    return runtime.http.json(200, {"count": len(users)})

# In background task
def worker():
    users = runtime.kv.get("users", default=[])
    print(f"Total users: {len(users)}")
```

## Notes

- All operations are thread-safe
- Values are deep-copied on retrieval
- TTL expiration is automatic
- Supports glob patterns for key matching
