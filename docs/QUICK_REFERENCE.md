# Scriptling Quick Reference

A practical cheat sheet for common Scriptling patterns and library usage.

## JSON Library

```python
import json

# Parse JSON string to objects
data = json.loads('{"name":"Alice","age":30}')
name = data["name"]  # "Alice"

# Convert objects to JSON string
obj = {"status": "success", "count": 42}
json_str = json.dumps(obj)
```

## Regex Library

```python
import re

# Match - returns boolean (matches at start of string only)
if re.match("[0-9]+", "123abc"):
    print("Starts with digits")

# Search - returns first match anywhere or None
email = re.search("[a-z]+@[a-z]+\.[a-z]+", "Contact: user@example.com")

# Find all - returns list
phones = re.findall("[0-9]{3}-[0-9]{4}", "555-1234 or 555-5678")

# Find all as Match objects - returns list of Match objects
matches = re.finditer("[0-9]{3}-[0-9]{4}", "555-1234 or 555-5678")
for match in matches:
    print(match.group(0))  # "555-1234", "555-5678"

# Sub - replacement (pattern, repl, string, count=0, flags=0)
text = re.sub("[0-9]+", "XXX", "Price: 100")
text = re.sub("[0-9]+", "X", "a1b2c3", 2)  # Replace only first 2

# Split - returns list (pattern, string, maxsplit=0, flags=0)
parts = re.split("[,;]", "one,two;three")

# Flags: re.I (IGNORECASE), re.M (MULTILINE), re.S (DOTALL)
if re.match("hello", "HELLO world", re.I):
    print("Case-insensitive match")
```

## Requests Library

Response: `status_code`, `text`, `headers`, `json()`, `raise_for_status()`
Supports both dict (`response["status_code"]`) and attribute access (`response.status_code`)

### Simple Requests (5 second default timeout)

```python
import requests

# GET with json() method
response = requests.get("https://api.example.com/users")
data = response.json()
print(data)

# GET with raise_for_status()
try:
    response = requests.get("https://api.example.com/data")
    response.raise_for_status()  # Raises error if 4xx/5xx
    data = response.json()
except Exception as e:
    print("Error:", e)

# Attribute access
if response.status_code == 200:
    content = response.text[:500]
    print(content)

# POST with data
import json
body = json.dumps({"name": "Alice"})
response = requests.post("https://api.example.com/users", data=body)

# Other methods
response = requests.put(url, data=body)
response = requests.delete(url)
response = requests.patch(url, data=body)
```

### With Options (timeout, headers, auth)

```python
# Using keyword arguments (Python-style)
response = requests.get(url, timeout=10)

response = requests.get(url, headers={
    "Authorization": "Bearer token123",
    "Accept": "application/json"
})

response = requests.post(url, data=body, timeout=10, headers={"Content-Type": "application/json"})

# Basic Authentication
response = requests.get(url, auth=("user", "pass"))
```

### Complete REST API Example

```python
import json
import requests

# Configure options
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123"}
}

# GET request
response = requests.get("https://api.example.com/users/1", options)

if response.status_code == 200:
    user = json.loads(response.body)
    print("User: " + user["name"])

    # Update user
    user["email"] = "new@example.com"
    body = json.dumps(user)

    update = requests.put("https://api.example.com/users/1", body, options)
    if update.status_code == 200:
        print("Updated!")
else:
    print("Error: " + str(response.status_code))
```

## Itertools Library

```python
import itertools

# Combining iterables
itertools.chain([1, 2], [3, 4])           # [1, 2, 3, 4]
itertools.zip_longest([1, 2], [3], fillvalue=0)  # [[1, 3], [2, 0]]

# Infinite iterators (use with count limit)
itertools.count(10)                        # [10, 11, 12, ...]
itertools.cycle([1, 2])                    # [1, 2, 1, 2, ...]
itertools.repeat("x", 3)                   # ["x", "x", "x"]

# Filtering
itertools.takewhile(lambda x: x < 3, [1, 2, 3, 2, 1])  # [1, 2]
itertools.dropwhile(lambda x: x < 3, [1, 2, 3, 2, 1])  # [3, 2, 1]
itertools.filterfalse(lambda x: x % 2, [1, 2, 3, 4])   # [2, 4]
itertools.compress([1, 2, 3, 4], [1, 0, 1, 0])         # [1, 3]

# Slicing and batching
itertools.islice([0, 1, 2, 3, 4], 1, 4)    # [1, 2, 3]
itertools.batched([1, 2, 3, 4, 5], 2)      # [[1, 2], [3, 4], [5]]
itertools.pairwise([1, 2, 3, 4])           # [[1, 2], [2, 3], [3, 4]]

# Combinatorics
itertools.permutations([1, 2, 3])          # All orderings
itertools.combinations([1, 2, 3], 2)       # All pairs
itertools.product([1, 2], ["a", "b"])      # Cartesian product

# Accumulate
itertools.accumulate([1, 2, 3, 4])         # [1, 3, 6, 10]
```

## Collections Library

```python
import collections

# Counter - count element occurrences
counter = collections.Counter([1, 1, 2, 3, 3, 3])  # {1: 2, 2: 1, 3: 3}
collections.most_common(counter, 2)                 # [(3, 3), (1, 2)]

# deque - double-ended queue
d = collections.deque([1, 2, 3])
collections.deque_appendleft(d, 0)          # [0, 1, 2, 3]
collections.deque_popleft(d)                # Returns 0, d is [1, 2, 3]
collections.deque_rotate(d, 1)              # Rotate right

# namedtuple - factory for dict with named fields
Point = collections.namedtuple("Point", ["x", "y"])
p = Point(1, 2)
print(p.x, p.y)                                 # 1 2

# ChainMap - merge multiple dicts
defaults = {"a": 1, "b": 2}
overrides = {"b": 20, "c": 3}
cm = collections.ChainMap(overrides, defaults)
cm["a"]                                     # 1 (from defaults)
cm["b"]                                     # 20 (from overrides)

# defaultdict - dict with default factory
d = collections.defaultdict(list)
d["key"].append(1)                              # Creates [] and appends 1
```

## Math Library

```python
import math

# Basic operations
math.sqrt(16)           # 4.0
math.pow(2, 8)          # 256.0
math.log(100)           # 4.605... (natural log)
math.log10(100)         # 2.0 (base 10)

# Trigonometric
math.sin(math.pi / 2)   # 1.0
math.cos(0)             # 1.0
math.tan(math.pi / 4)   # 1.0

# Rounding
math.floor(3.7)         # 3
math.ceil(3.2)          # 4

# Constants
math.pi                 # 3.14159...
math.e                  # 2.71828...
```

## Random Library

```python
import random

# Basic random values
random.random()          # Float between 0 and 1
random.randint(1, 10)   # Integer between 1 and 10 (inclusive)
random.uniform(1.0, 10.0)  # Float between 1.0 and 10.0

# Choices from sequences
items = ["apple", "banana", "cherry"]
random.choice(items)    # Single random element
random.sample(items, 2) # 2 unique elements

# Shuffling
deck = [1, 2, 3, 4, 5]
random.shuffle(deck)    # Modifies in-place
```

## Time & Date

```python
import time
import datetime

# Current time
now = time.time()               # Unix timestamp (seconds since epoch)
formatted = datetime.now()      # Formatted date string

# Sleep
time.sleep(1)                   # Sleep for 1 second

# Parse/format
dt = datetime.datetime("2024-01-15 10:30:00", "%Y-%m-%d %H:%M:%S")
ts = datetime.timestamp(dt)     # Convert to timestamp
```

## Base64 Encoding

```python
import base64

# Encode
encoded = base64.encode("Hello, World!")
print(encoded)  # "SGVsbG8sIFdvcmxkIQ=="

# Decode
decoded = base64.decode("SGVsbG8sIFdvcmxkIQ==")
print(decoded)  # "Hello, World!"
```

## Hash Functions

```python
import hashlib

# MD5
md5_hash = hashlib.md5("data")
print(md5_hash)  # Hex string

# SHA256
sha256_hash = hashlib.sha256("data")
print(sha256_hash)  # 64 character hex string

# SHA512
sha512_hash = hashlib.sha512("data")
print(sha512_hash)  # 128 character hex string
```

## String Library (Constants)

```python
import string

# Character sets
string.ascii_letters      # "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
string.ascii_lowercase    # "abcdefghijklmnopqrstuvwxyz"
string.ascii_uppercase    # "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
string.digits             # "0123456789"
string.hexdigits         # "0123456789abcdefABCDEF"
string.punctuation        # Punctuation characters
string.whitespace         # Space, tab, newline, etc.
```

## UUID Generation

```python
import uuid

# Random UUID
id = uuid.uuid4()
print(id)  # e.g., "f47ac10b-58cc-4372-a567-0e02b2c3d479"
```

## Platform Information

```python
import platform

# System info
platform.system()        # "Linux", "Darwin", "Windows"
platform.machine()       # "x86_64", "arm64"
```

## Common Patterns

### HTTP Error Handling

```python
import requests
import json

url = "https://api.example.com/data"
options = {"timeout": 10}

try:
    response = requests.get(url, options)
    if response.status_code == 200:
        data = json.loads(response.body)
        print("Success:", len(data))
    else:
        print("HTTP Error:", response.status_code)
except Exception as e:
    print("Request failed:", e)
```

### Retry Pattern

```python
import requests
import time

def fetch_with_retry(url, max_retries=3):
    options = {"timeout": 5}
    for i in range(max_retries):
        response = requests.get(url, options)
        if response.status_code == 200:
            return response.body
        time.sleep(1)  # Wait before retry
    return None
```

### Data Processing Pipeline

```python
import json
import requests

# Fetch
response = requests.get("https://api.example.com/items", {"timeout": 10})

# Parse
data = json.loads(response.body)

# Filter
filtered = [item for item in data if item["active"]]

# Transform
result = [{"id": x["id"], "name": x["name"].upper()} for x in filtered]

# Output
print(json.dumps(result))
```

### Batch Processing with Itertools

```python
import itertools
import requests

def process_batch(items):
    # Process a batch of items
    return [{"processed": True, "item": x} for x in items]

# Fetch items
response = requests.get("https://api.example.com/items")
items = json.loads(response.body)

# Process in batches of 100
for batch in itertools.batched(items, 100):
    results = process_batch(batch)
    print(f"Processed {len(results)} items")
```

### URL Handling

```python
import urllib.parse

# Parse URL
url = "https://example.com/path?key=value&foo=bar"
parsed = urllib.parse.urlparse(url)
print(parsed.scheme)   # "https"
print(parsed.netloc)   # "example.com"
print(parsed.path)     # "/path"
print(parsed.query)    # "key=value&foo=bar"

# Parse query string
query = urllib.parse.parse_qs("key=value&foo=bar")
print(query["key"])    # ["value"]

# Build URL
params = {"key": "value", "foo": "bar"}
query_string = urllib.parse.urlencode(params)
url = "https://example.com/api?" + query_string
```

## Language Basics

For complete language reference (types, operators, control flow, functions, classes), see [LANGUAGE_GUIDE.md](LANGUAGE_GUIDE.md).

Quick reminders:

```python
# Variables and types
x = 42
name = "Alice"
items = [1, 2, 3]
data = {"key": "value"}

# Control flow
if x > 10:
    print("large")
elif x > 5:
    print("medium")
else:
    print("small")

# Loops
for i in range(5):
    print(i)

for item in items:
    print(item)

# Functions
def add(a, b):
    return a + b

# Error handling
try:
    result = risky_operation()
except:
    result = None
finally:
    cleanup()

# Assert
assert x > 0, "x must be positive"
```

## See Also

- [LANGUAGE_GUIDE.md](LANGUAGE_GUIDE.md) - Complete syntax and language reference
- [LIBRARIES.md](LIBRARIES.md) - All available libraries
