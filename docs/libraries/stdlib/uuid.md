# uuid Library

UUID generation library, matching Python's `uuid` module.

## Import

```python
import uuid
```

## Available Functions

| Function  | Description                                   |
| --------- | --------------------------------------------- |
| `uuid1()` | Generate UUID version 1 (time-based)          |
| `uuid4()` | Generate UUID version 4 (random)              |
| `uuid7()` | Generate UUID version 7 (timestamp, sortable) |

## Functions

### `uuid1()`

Generate a UUID version 1 (time-based).

Based on current time and MAC address. Good for generating unique IDs where time ordering matters.

```python
id = uuid.uuid1()
print(id)  # e.g., "f47ac10b-58cc-1e4c-a26f-e3fc32165abc"
```

### `uuid4()`

Generate a UUID version 4 (random).

Randomly generated UUID. Most commonly used for general-purpose unique identifiers.

```python
id = uuid.uuid4()
print(id)  # e.g., "550e8400-e29b-41d4-a716-446655440000"
```

### `uuid7()`

Generate a UUID version 7 (Unix timestamp-based, sortable).

Based on Unix timestamp in milliseconds. UUIDs generated in sequence will sort in chronological order. Ideal for database primary keys.

```python
id = uuid.uuid7()
print(id)  # e.g., "018f6b1c-4e5d-7abc-8def-0123456789ab"
```

## UUID Versions Comparison

| Version | Based On   | Sortable  | Use Case                           |
| ------- | ---------- | --------- | ---------------------------------- |
| uuid1() | Time + MAC | Partially | Legacy systems, audit trails       |
| uuid4() | Random     | No        | General purpose, most common       |
| uuid7() | Timestamp  | Yes       | Database keys, distributed systems |

## Examples

### Generate Unique Request ID

```python
import uuid

def make_request():
    request_id = uuid.uuid4()
    print("Request ID:", request_id)
    # Use request_id for tracking...

make_request()
```

### Database Record ID

```python
import uuid

def create_record(data):
    record = {
        "id": uuid.uuid7(),  # Sortable by creation time
        "data": data
    }
    return record

record = create_record({"name": "Test"})
print(record["id"])
```

### Batch Processing with Ordered IDs

```python
import uuid
import time

ids = []
for i in range(3):
    ids.append(uuid.uuid7())
    time.sleep(0.01)  # Small delay

# IDs will be in chronological order
for id in ids:
    print(id)
```

## Python Compatibility

- `uuid1()` - ✅ Compatible
- `uuid4()` - ✅ Compatible
- `uuid7()` - ✅ Compatible (Python 3.12+)

Note: Python's uuid module also provides `uuid3()` and `uuid5()` (name-based) which are not currently implemented.
