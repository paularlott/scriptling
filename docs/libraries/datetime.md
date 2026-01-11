# Datetime Library

Datetime functions for formatting and parsing dates and times. Python-compatible.

```python
import datetime
```

## Functions

### datetime.now(format?)

Returns the current local date and time as a formatted string.

**Parameters:**
- `format` (optional): Python-style format string (default: "%Y-%m-%d %H:%M:%S")

**Returns:** String

**Example:**
```python
import datetime

# Current datetime
now = datetime.now()  # "2025-11-26 11:15:54"

# Custom format
now = datetime.now("%Y-%m-%d %H:%M:%S")  # "2025-11-26 11:15:54"
```

### datetime.utcnow(format?)

Returns the current UTC date and time as a formatted string.

**Parameters:**
- `format` (optional): Python-style format string (default: "%Y-%m-%d %H:%M:%S")

**Returns:** String

**Example:**
```python
import datetime

# Current UTC datetime
utc_now = datetime.utcnow()  # "2025-11-26 03:15:54"

# Custom format
utc_now = datetime.utcnow("%Y-%m-%d %H:%M:%S")  # "2025-11-26 03:15:54"
```

### datetime.today(format?)

Returns today's date as a formatted string.

**Parameters:**
- `format` (optional): Python-style format string (default: "%Y-%m-%d")

**Returns:** String

**Example:**
```python
import datetime

# Today's date
today = datetime.today()  # "2025-11-26"

# Custom format
today = datetime.today("%A, %B %d, %Y")  # "Wednesday, November 26, 2025"
```

### datetime.strptime(date_string, format)

Parses a date string according to the given format and returns a Unix timestamp.

**Parameters:**
- `date_string`: String to parse
- `format`: Python-style format string

**Returns:** Float (Unix timestamp)

**Example:**
```python
import datetime

timestamp = datetime.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S")
# Returns: 1705314645.0
```

### datetime.strftime(format, timestamp)

Formats a Unix timestamp according to the given format string.

**Parameters:**
- `format`: Python-style format string
- `timestamp`: Unix timestamp (integer or float)

**Returns:** String

**Example:**
```python
import datetime

formatted = datetime.strftime("%Y-%m-%d %H:%M:%S", 1705314645.0)
# Returns: "2024-01-15 18:30:45"
```

### datetime.fromtimestamp(timestamp, format?)

Creates a formatted datetime string from a Unix timestamp.

**Parameters:**
- `timestamp`: Unix timestamp (integer or float)
- `format` (optional): Python-style format string (default: "%Y-%m-%d %H:%M:%S")

**Returns:** String

**Example:**
```python
import datetime

dt = datetime.fromtimestamp(1705314645.0)
# Returns: "2024-01-15 18:30:45"

dt = datetime.fromtimestamp(1705314645.0, "%A, %B %d, %Y at %I:%M %p")
# Returns: "Monday, January 15, 2024 at 06:30 PM"
```

### datetime.isoformat(timestamp?)

Returns the date and time in ISO 8601 format.

**Parameters:**
- `timestamp` (optional): Unix timestamp (integer or float). Defaults to current time.

**Returns:** String

**Example:**
```python
import datetime

# Current time in ISO format
iso = datetime.isoformat()
# Returns: "2025-11-26T12:15:30Z"

# Specific timestamp in ISO format
iso = datetime.isoformat(1705314645.0)
# Returns: "2024-01-15T18:30:45Z"
```

### datetime.timestamp()

Returns the current Unix timestamp.

**Parameters:** None

**Returns:** Float (Unix timestamp)

**Example:**
```python
import datetime

ts = datetime.timestamp()
# Returns: 1732622130.0 (current time as Unix timestamp)
```

### datetime.timedelta(days=0, seconds=0, microseconds=0, milliseconds=0, minutes=0, hours=0, weeks=0)

Creates a timedelta representing a duration and returns the total seconds.

**Parameters (all optional, keyword-only):**
- `days`: Number of days
- `seconds`: Number of seconds
- `microseconds`: Number of microseconds
- `milliseconds`: Number of milliseconds
- `minutes`: Number of minutes
- `hours`: Number of hours
- `weeks`: Number of weeks

**Returns:** Float (total duration in seconds)

**Example:**
```python
import datetime

# One day in seconds
one_day = datetime.timedelta(days=1)
# Returns: 86400.0

# Two hours
two_hours = datetime.timedelta(hours=2)
# Returns: 7200.0

# Combined duration
duration = datetime.timedelta(days=1, hours=2, minutes=30)
# Returns: 95400.0

# Use with timestamps
now = datetime.timestamp()
tomorrow = now + datetime.timedelta(days=1)
next_week = now + datetime.timedelta(weeks=1)
```

## Format Codes

| Code | Description | Example |
|------|-------------|---------|
| `%Y` | Year (4 digits) | 2024 |
| `%m` | Month (01-12) | 01 |
| `%d` | Day (01-31) | 15 |
| `%H` | Hour (00-23) | 18 |
| `%I` | Hour (01-12) | 06 |
| `%M` | Minute (00-59) | 30 |
| `%S` | Second (00-59) | 45 |
| `%A` | Full weekday | Monday |
| `%a` | Abbreviated weekday | Mon |
| `%B` | Full month | January |
| `%b` | Abbreviated month | Jan |
| `%p` | AM/PM | PM |
| `%Z` | Timezone name | MST |
| `%z` | Timezone offset | -0700 |

## Notes

- All functions return strings except `timestamp()` and `strptime()` which return floats
- Timestamps are Unix timestamps (seconds since 1970-01-01 00:00:00 UTC)
- Format codes follow Python's strftime/strptime conventions
- Date arithmetic should use standard arithmetic on Unix timestamps (e.g., `ts + 86400` for one day)