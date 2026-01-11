# Scriptling Libraries

Scriptling provides a rich set of standard libraries and supports loading external libraries on demand.

## Core Functions

Always available without importing:

### I/O
- `print(value)` - Output to console

### Type Conversions
- `str(value)` - Convert to string
- `int(value)` - Convert to integer
- `float(value)` - Convert to float
- `bool(value)` - Convert to boolean
- `list(value)` - Convert to list
- `dict(value)` - Convert to dictionary

### String Functions
- `len(string)` - Get length
- `upper(string)` - Uppercase
- `lower(string)` - Lowercase
- `capitalize(string)` - Capitalize first letter
- `title(string)` - Title case
- `split(string, sep)` - Split to list
- `join(list, sep)` - Join from list
- `replace(str, old, new)` - Replace substring
- `strip(string)` - Trim whitespace from both ends
- `lstrip(string)` - Trim whitespace from left
- `rstrip(string)` - Trim whitespace from right
- `startswith(string, prefix)` - Check if string starts with prefix
- `endswith(string, suffix)` - Check if string ends with suffix
- `find(string, sub)` - Find substring index
- `count(string, sub)` - Count occurrences

### List Functions
- `len(list)` - Get length
- `list.append(item)` - Append item (modifies list in-place)
- `list.extend(other_list)` - Append elements from another list (modifies list in-place)
- `list.pop(index)` - Remove and return item at index
- `list.insert(index, item)` - Insert item at index
- `list.remove(item)` - Remove first occurrence of item
- `list.index(item)` - Return index of item
- `list.count(item)` - Count occurrences of item
- `list.sort()` - Sort list in-place
- `list.reverse()` - Reverse list in-place

### Dictionary Functions
- `dict.keys()` - Get view of keys (iterable, reflects dict changes)
- `dict.values()` - Get view of values (iterable, reflects dict changes)
- `dict.items()` - Get view of (key, value) tuples (iterable, reflects dict changes)
- `dict.get(key, default)` - Get value with default
- `dict.pop(key, default)` - Remove and return value
- `dict.update(other)` - Update with other dictionary

**Note:** `keys()`, `values()`, and `items()` now return view objects instead of lists. Use `list(dict.keys())` if you need a list.

### System
- `import library_name` - Load library dynamically
- `help([object])` - Display help information for functions, libraries, and objects
- `type(object)` - Get type of object
- `isinstance(object, type)` - Check if object is instance of type
- `dir(object)` - List attributes of object

## Standard Libraries

These libraries are built-in and available for import without any registration.

| Import | Description | Details |
|--------|-------------|---------|
| `json` | Parse and generate JSON data | [libraries/json.md](libraries/json.md) |
| `base64` | Base64 encoding and decoding | [libraries/base64.md](libraries/base64.md) |
| `html` | HTML escaping and unescaping | [libraries/html.md](libraries/html.md) |
| `math` | Mathematical functions and constants | [libraries/math.md](libraries/math.md) |
| `random` | Random number generation | [libraries/random.md](libraries/random.md) |
| `statistics` | Statistical functions | [libraries/statistics.md](libraries/statistics.md) |
| `time` | Time access and conversions | [libraries/time.md](libraries/time.md) |
| `datetime` | Date and time formatting | [libraries/datetime.md](libraries/datetime.md) |
| `re` | Regular expression operations | [libraries/regex.md](libraries/regex.md) |
| `string` | String constants | [libraries/string.md](libraries/string.md) |
| `textwrap` | Text wrapping and filling | [libraries/textwrap.md](libraries/textwrap.md) |
| `functools` | Higher-order functions | [libraries/functools.md](libraries/functools.md) |
| `itertools` | Iterator functions | [libraries/itertools.md](libraries/itertools.md) |
| `collections` | Specialized container datatypes | [libraries/collections.md](libraries/collections.md) |
| `hashlib` | Secure hash algorithms | [libraries/hashlib.md](libraries/hashlib.md) |
| `platform` | Platform identifying data | [libraries/platform.md](libraries/platform.md) |
| `urllib` | URL handling | [libraries/urllib.md](libraries/urllib.md) |
| `uuid` | UUID generation | [libraries/uuid.md](libraries/uuid.md) |

## Extended Libraries

These libraries require explicit registration by the host application (e.g., the CLI tool).

| Import | Description | Details |
|--------|-------------|---------|
| `requests` | HTTP library for sending requests | [libraries/requests.md](libraries/requests.md) |
| `sys` | System-specific parameters and functions | [libraries/sys.md](libraries/sys.md) |
| `secrets` | Cryptographically strong random numbers | [libraries/secrets.md](libraries/secrets.md) |
| `subprocess` | Spawn and manage subprocesses | [libraries/subprocess.md](libraries/subprocess.md) |
| `html.parser` | HTML/XHTML parser | [libraries/html.parser.md](libraries/html.parser.md) |
| `os` | Operating system interfaces (filesystem) | [libraries/os.md](libraries/os.md) |
| `os.path` | Pathname manipulations | [libraries/os.path.md](libraries/os.path.md) |
| `pathlib` | Object-oriented filesystem paths | [libraries/pathlib.md](libraries/pathlib.md) |
| `threads` | Asynchronous execution with isolated environments | [libraries/threads.md](libraries/threads.md) |
| `logging` | Logging functionality | [libraries/logging.md](libraries/logging.md) |
| `wait_for` | Wait for resources to become available | [libraries/wait_for.md](libraries/wait_for.md) |



## Usage Example

```python
import json
import math

data = json.loads('{"a": 1, "b": 2}')
print(math.sqrt(data["a"] + data["b"]))
```

For detailed documentation on each library, use the `help()` function within the interactive shell or script:

```python
import json
help(json)
```
