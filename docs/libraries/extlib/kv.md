# scriptling.kv

Thread-safe key-value store for sharing state across HTTP requests.

## Overview

The `scriptling.kv` library provides an in-memory key-value store that persists across HTTP requests. It's perfect for:

- Session management
- Rate limiting counters
- Caching API responses
- Storing user state
- Sharing data between request handlers

**Key features:**

- Thread-safe operations
- Optional TTL (time-to-live) for keys
- Deep copy on read (modifying returned values doesn't affect stored data)
- Atomic increment operations
- Glob pattern matching for keys

## Functions

### scriptling.kv.set(key, value, ttl=0)

Store a value with optional TTL in seconds.

**Parameters:**
- `key` (str): The key to store the value under
- `value`: The value to store (string, int, float, bool, list, dict)
- `ttl` (int, optional): Time-to-live in seconds. 0 means no expiration.

**Returns:** None

**Example:**
```python
import scriptling.kv

# Store a simple value
scriptling.kv.set("api_key", "secret123")

# Store with TTL (1 hour)
scriptling.kv.set("session:abc", {"user": "bob", "login_time": 123456}, ttl=3600)

# Store a list
scriptling.kv.set("recent_items", ["item1", "item2", "item3"])

# Store a dict
scriptling.kv.set("config", {"debug": True, "port": 8080})
```

### scriptling.kv.get(key, default=None)

Retrieve a value by key.

**Parameters:**
- `key` (str): The key to retrieve
- `default`: Value to return if key doesn't exist (default: None)

**Returns:** The stored value (deep copy), or the default if not found

**Example:**
```python
import scriptling.kv

# Get a value
value = scriptling.kv.get("api_key")

# Get with default
count = scriptling.kv.get("counter", default=0)

# Expired keys return default
session = scriptling.kv.get("session:old", default={})
```

### scriptling.kv.delete(key)

Remove a key from the store.

**Parameters:**
- `key` (str): The key to delete

**Returns:** None

**Example:**
```python
import scriptling.kv

scriptling.kv.delete("session:abc")
scriptling.kv.delete("temp_data")
```

### scriptling.kv.exists(key)

Check if a key exists and is not expired.

**Parameters:**
- `key` (str): The key to check

**Returns:** bool - True if key exists and is not expired

**Example:**
```python
import scriptling.kv

if scriptling.kv.exists("config"):
    config = scriptling.kv.get("config")
else:
    config = load_default_config()
```

### scriptling.kv.incr(key, amount=1)

Atomically increment an integer value.

**Parameters:**
- `key` (str): The key to increment
- `amount` (int, optional): Amount to increment by (default: 1)

**Returns:** int - The new value after incrementing

**Example:**
```python
import scriptling.kv

# Initialize counter
scriptling.kv.set("page_views", 0)

# Increment (returns 1)
views = scriptling.kv.incr("page_views")

# Increment by more (returns 6)
views = scriptling.kv.incr("page_views", 5)

# Non-existent keys are auto-initialized
scriptling.kv.incr("new_counter")  # returns 1
```

### scriptling.kv.ttl(key)

Get remaining time-to-live for a key.

**Parameters:**
- `key` (str): The key to check

**Returns:** int - Remaining TTL in seconds, -1 if no expiration, -2 if key doesn't exist

**Example:**
```python
import scriptling.kv

scriptling.kv.set("session", "data", ttl=3600)

remaining = scriptling.kv.ttl("session")  # e.g., 3599
no_expiry = scriptling.kv.ttl("config")  # -1 (no TTL)
missing = scriptling.kv.ttl("unknown")   # -2 (doesn't exist)
```

### scriptling.kv.keys(pattern="*")

Get all keys matching a glob pattern.

**Parameters:**
- `pattern` (str, optional): Glob pattern to match keys (default: "*")

**Returns:** list - List of matching keys

**Example:**
```python
import scriptling.kv

# Get all keys
all_keys = scriptling.kv.keys()

# Get keys matching pattern
user_keys = scriptling.kv.keys("user:*")
session_keys = scriptling.kv.keys("session:*")
cache_keys = scriptling.kv.keys("cache:*:data")
```

### scriptling.kv.clear()

Remove all keys from the store.

**Warning:** This operation cannot be undone.

**Returns:** None

**Example:**
```python
import scriptling.kv

# Clear all data (use with caution!)
scriptling.kv.clear()
```

## Usage Examples

### Rate Limiting

```python
import scriptling.http
import scriptling.kv

def check_rate_limit(request):
    ip = request.headers.get("x-forwarded-for", "unknown")
    key = f"rate:{ip}"

    count = scriptling.kv.incr(key)

    # Set expiry on first request
    if count == 1:
        scriptling.kv.set(key, 1, ttl=60)  # 1 minute window

    if count > 100:
        return scriptling.http.json(429, {"error": "Rate limit exceeded"})

    return None  # Allow request
```

### Session Management

```python
import scriptling.http
import scriptling.kv
import secrets

def create_session(user_id):
    token = secrets.token_hex(32)
    scriptling.kv.set(f"session:{token}", {
        "user_id": user_id,
        "created": time.time()
    }, ttl=86400)  # 24 hours
    return token

def get_session(token):
    return scriptling.kv.get(f"session:{token}")

def delete_session(token):
    scriptling.kv.delete(f"session:{token}")
```

### Caching API Responses

```python
import scriptling.http
import scriptling.kv
import requests
import json

def get_cached_api_data(endpoint, ttl=300):
    cache_key = f"api:{endpoint}"

    # Try cache first
    cached = scriptling.kv.get(cache_key)
    if cached:
        return cached

    # Fetch from API
    response = requests.get(f"https://api.example.com/{endpoint}")
    data = response.json()

    # Cache for future requests
    scriptling.kv.set(cache_key, data, ttl=ttl)

    return data
```

## See Also

- [scriptling.http](http.md) - HTTP server route registration
- [requests](requests.md) - HTTP client library
