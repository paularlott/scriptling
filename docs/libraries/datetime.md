# Datetime Library

Datetime functions for formatting and parsing dates and times. Requires import.

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

### datetime.add_days(timestamp, days)

Adds or subtracts days from a Unix timestamp.

**Parameters:**
- `timestamp`: Unix timestamp (integer or float)
- `days`: Number of days to add (positive) or subtract (negative) (integer or float)

**Returns:** Float (new Unix timestamp)

**Example:**
```python
import datetime

original = 1705314645.0  # 2024-01-15 10:30:45 UTC
future = datetime.add_days(original, 7)      # +7 days
past = datetime.add_days(original, -3)       # -3 days

print(datetime.fromtimestamp(original))  # "2024-01-15 18:30:45"
print(datetime.fromtimestamp(future))    # "2024-01-22 18:30:45"
print(datetime.fromtimestamp(past))      # "2024-01-12 18:30:45"
```

### datetime.add_hours(timestamp, hours)

Adds or subtracts hours from a Unix timestamp.

**Parameters:**
- `timestamp`: Unix timestamp (integer or float)
- `hours`: Number of hours to add (positive) or subtract (negative) (integer or float)

**Returns:** Float (new Unix timestamp)

**Example:**
```python
import datetime

original = 1705314645.0  # 2024-01-15 10:30:45 UTC
later = datetime.add_hours(original, 5)     # +5 hours
earlier = datetime.add_hours(original, -2)  # -2 hours

print(datetime.fromtimestamp(original))  # "2024-01-15 18:30:45"
print(datetime.fromtimestamp(later))     # "2024-01-15 23:30:45"
print(datetime.fromtimestamp(earlier))   # "2024-01-15 16:30:45"
```

### datetime.add_minutes(timestamp, minutes)

Adds or subtracts minutes from a Unix timestamp.

**Parameters:**
- `timestamp`: Unix timestamp (integer or float)
- `minutes`: Number of minutes to add (positive) or subtract (negative) (integer or float)

**Returns:** Float (new Unix timestamp)

**Example:**
```python
import datetime

original = 1705314645.0  # 2024-01-15 10:30:45 UTC
later = datetime.add_minutes(original, 30)    # +30 minutes
earlier = datetime.add_minutes(original, -15) # -15 minutes

print(datetime.fromtimestamp(original))  # "2024-01-15 18:30:45"
print(datetime.fromtimestamp(later))     # "2024-01-15 19:00:45"
print(datetime.fromtimestamp(earlier))   # "2024-01-15 18:15:45"
```

### datetime.add_seconds(timestamp, seconds)

Adds or subtracts seconds from a Unix timestamp.

**Parameters:**
- `timestamp`: Unix timestamp (integer or float)
- `seconds`: Number of seconds to add (positive) or subtract (negative) (integer or float)

**Returns:** Float (new Unix timestamp)

**Example:**
```python
import datetime

original = 1705314645.0  # 2024-01-15 10:30:45 UTC
later = datetime.add_seconds(original, 45)    # +45 seconds
earlier = datetime.add_seconds(original, -30) # -30 seconds

print(datetime.fromtimestamp(original))  # "2024-01-15 18:30:45"
print(datetime.fromtimestamp(later))     # "2024-01-15 18:31:30"
print(datetime.fromtimestamp(earlier))   # "2024-01-15 18:30:15"
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

## Notes

- All functions return strings, not datetime objects
- Timestamps are Unix timestamps (seconds since 1970-01-01 00:00:00 UTC)
- Format codes follow Python's strftime/strptime conventions
- These functions are always available - no import required