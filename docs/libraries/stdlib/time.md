# Time Library

Time-related functions for timestamps, time tuples, formatting, and sleeping. Python-compatible implementation.

```python
import time
```

## Available Functions

| Function                   | Description                                   |
| -------------------------- | --------------------------------------------- |
| `time()`                   | Returns the current Unix timestamp            |
| `perf_counter()`           | Returns a high-resolution monotonic timer     |
| `sleep(seconds)`           | Pauses execution for the specified seconds    |
| `localtime([secs])`        | Converts timestamp to local time tuple        |
| `gmtime([secs])`           | Converts timestamp to UTC time tuple          |
| `mktime(tuple)`            | Converts time tuple to Unix timestamp         |
| `strftime(format[, t])`    | Formats a time tuple as a string              |
| `strptime(string, format)` | Parses a string to a time tuple               |
| `asctime([t])`             | Converts time tuple to standard format string |
| `ctime([secs])`            | Converts timestamp to standard format string  |

## Functions

### time.time()

Returns the current Unix timestamp (seconds since epoch as float).

**Returns:** Float

**Example:**

```python
import time
now = time.time()  # 1732435200.123456
```

### time.perf_counter()

Returns a high-resolution monotonic timer for benchmarking (seconds since program start).

**Returns:** Float

**Example:**

```python
import time

start = time.perf_counter()
# ... some code ...
end = time.perf_counter()
elapsed = end - start
```

### time.sleep(seconds)

Pauses execution for the specified number of seconds.

**Parameters:**

- `seconds`: Number of seconds to sleep (integer or float)

**Returns:** None

**Example:**

```python
import time

print("Waiting...")
time.sleep(1)      # Sleep 1 second
time.sleep(0.5)    # Sleep 0.5 seconds
print("Done!")
```

### time.localtime([secs])

Converts a Unix timestamp to a time tuple representing local time.

**Parameters:**

- `secs` (optional): Unix timestamp (integer or float). Defaults to current time.

**Returns:** List (9-element time tuple: [year, month, day, hour, minute, second, weekday, yearday, dst])

**Example:**

```python
import time

# Current local time tuple
local_tuple = time.localtime()
# [2025, 11, 26, 11, 58, 18, 3, 330, 0]

# Specific timestamp
specific_tuple = time.localtime(1705314645.0)
# [2024, 1, 15, 18, 30, 45, 1, 15, 0]
```

### time.gmtime([secs])

Converts a Unix timestamp to a time tuple representing UTC time.

**Parameters:**

- `secs` (optional): Unix timestamp (integer or float). Defaults to current time.

**Returns:** List (9-element time tuple: [year, month, day, hour, minute, second, weekday, yearday, dst])

**Example:**

```python
import time

# Current UTC time tuple
utc_tuple = time.gmtime()
# [2025, 11, 26, 3, 58, 18, 3, 330, 0]

# Specific UTC timestamp
utc_specific = time.gmtime(1705314645.0)
# [2024, 1, 15, 18, 30, 45, 1, 15, 0]
```

### time.mktime(tuple)

Converts a time tuple back to a Unix timestamp.

**Parameters:**

- `tuple`: 9-element time tuple (list)

**Returns:** Float (Unix timestamp)

**Example:**

```python
import time

tuple = [2024, 1, 15, 18, 30, 45, 1, 15, 0]
timestamp = time.mktime(tuple)
# 1705314645.0
```

### time.strftime(format[, t])

Formats a time tuple according to the given format string.

**Parameters:**

- `format`: Python-style format string
- `t` (optional): 9-element time tuple. Defaults to current local time.

**Returns:** String

**Example:**

```python
import time

# Format current time
formatted = time.strftime("%Y-%m-%d %H:%M:%S")
# "2025-11-26 11:58:18"

# Format specific time tuple
tuple = time.localtime(1705314645.0)
formatted = time.strftime("%Y-%m-%d %H:%M:%S", tuple)
# "2024-01-15 18:30:45"
```

### time.strptime(string, format)

Parses a time string according to the given format and returns a time tuple.

**Parameters:**

- `string`: Time string to parse
- `format`: Python-style format string

**Returns:** List (9-element time tuple)

**Example:**

```python
import time

tuple = time.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S")
# [2024, 1, 15, 10, 30, 45, 1, 15, 0]
```

### time.asctime([t])

Converts a time tuple to a string in a standard format.

**Parameters:**

- `t` (optional): 9-element time tuple. Defaults to current local time.

**Returns:** String

**Example:**

```python
import time

# Current time
ascii_time = time.asctime()
# "Wed Nov 26 11:58:18 2025"

# Specific time tuple
tuple = time.localtime(1705314645.0)
ascii_time = time.asctime(tuple)
# "Mon Jan 15 18:30:45 2024"
```

### time.ctime([secs])

Converts a Unix timestamp to a string in a standard format.

**Parameters:**

- `secs` (optional): Unix timestamp. Defaults to current time.

**Returns:** String

**Example:**

```python
import time

# Current time
ctime_str = time.ctime()
# "Wed Nov 26 11:58:18 2025"

# Specific timestamp
ctime_str = time.ctime(1705314645.0)
# "Mon Jan 15 18:30:45 2024"
```

## Time Tuple Format

Time tuples are 9-element lists with the following structure:

```python
[year, month, day, hour, minute, second, weekday, yearday, dst]
```

- `year`: Year (e.g., 2025)
- `month`: Month (1-12)
- `day`: Day of month (1-31)
- `hour`: Hour (0-23)
- `minute`: Minute (0-59)
- `second`: Second (0-59)
- `weekday`: Day of week (0=Monday, 6=Sunday)
- `yearday`: Day of year (1-366)
- `dst`: Daylight saving time flag (0=not DST, 1=DST, -1=unknown)

## Format Codes

| Code | Description         | Example |
| ---- | ------------------- | ------- |
| `%Y` | Year (4 digits)     | 2024    |
| `%m` | Month (01-12)       | 01      |
| `%d` | Day (01-31)         | 15      |
| `%H` | Hour (00-23)        | 18      |
| `%M` | Minute (00-59)      | 30      |
| `%S` | Second (00-59)      | 45      |
| `%A` | Full weekday        | Monday  |
| `%a` | Abbreviated weekday | Mon     |
| `%B` | Full month          | January |
| `%b` | Abbreviated month   | Jan     |
| `%p` | AM/PM               | PM      |

## Usage Examples

```python
import time

# Get current timestamp
now = time.time()  # 1732435200.123456

# Convert to time tuple
local_tuple = time.localtime(now)
# [2025, 11, 26, 11, 58, 18, 3, 330, 0]

# Format as string
formatted = time.strftime("%Y-%m-%d %H:%M:%S", local_tuple)
# "2025-11-26 11:58:18"

# Parse string back to tuple
parsed = time.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S")
# [2024, 1, 15, 10, 30, 45, 1, 15, 0]

# Convert back to timestamp
timestamp = time.mktime(parsed)
# 1705314645.0

# Sleep
print("Waiting...")
time.sleep(1)
print("Done!")
```
