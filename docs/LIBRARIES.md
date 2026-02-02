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
- `strip(string[, chars])` - Trim whitespace (or specified chars) from both ends
- `lstrip(string[, chars])` - Trim whitespace (or specified chars) from left
- `rstrip(string[, chars])` - Trim whitespace (or specified chars) from right
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

| Import        | Description                          | Details                                                            |
| ------------- | ------------------------------------ | ------------------------------------------------------------------ |
| `json`        | Parse and generate JSON data         | [libraries/stdlib/json.md](libraries/stdlib/json.md)               |
| `base64`      | Base64 encoding and decoding         | [libraries/stdlib/base64.md](libraries/stdlib/base64.md)           |
| `html`        | HTML escaping and unescaping         | [libraries/stdlib/html.md](libraries/stdlib/html.md)               |
| `math`        | Mathematical functions and constants | [libraries/stdlib/math.md](libraries/stdlib/math.md)               |
| `random`      | Random number generation             | [libraries/stdlib/random.md](libraries/stdlib/random.md)           |
| `statistics`  | Statistical functions                | [libraries/stdlib/statistics.md](libraries/stdlib/statistics.md)   |
| `time`        | Time access and conversions          | [libraries/stdlib/time.md](libraries/stdlib/time.md)               |
| `datetime`    | Date and time formatting             | [libraries/stdlib/datetime.md](libraries/stdlib/datetime.md)       |
| `re`          | Regular expression operations        | [libraries/stdlib/regex.md](libraries/stdlib/regex.md)             |
| `string`      | String constants                     | [libraries/stdlib/string.md](libraries/stdlib/string.md)           |
| `textwrap`    | Text wrapping and filling            | [libraries/stdlib/textwrap.md](libraries/stdlib/textwrap.md)       |
| `functools`   | Higher-order functions               | [libraries/stdlib/functools.md](libraries/stdlib/functools.md)     |
| `itertools`   | Iterator functions                   | [libraries/stdlib/itertools.md](libraries/stdlib/itertools.md)     |
| `collections` | Specialized container datatypes      | [libraries/stdlib/collections.md](libraries/stdlib/collections.md) |
| `hashlib`     | Secure hash algorithms               | [libraries/stdlib/hashlib.md](libraries/stdlib/hashlib.md)         |
| `platform`    | Platform identifying data            | [libraries/stdlib/platform.md](libraries/stdlib/platform.md)       |
| `urllib`      | URL handling                         | [libraries/stdlib/urllib.md](libraries/stdlib/urllib.md)           |
| `uuid`        | UUID generation                      | [libraries/stdlib/uuid.md](libraries/stdlib/uuid.md)               |

## Scriptling Libraries

These are scriptling-specific libraries that provide functionality not available in Python's standard library. They use the `scriptling.` namespace prefix.

| Import                  | Description                                             | Details                                                            |
| ----------------------- | ------------------------------------------------------- | ------------------------------------------------------------------ |
| `scriptling.ai`         | AI and LLM functions for OpenAI-compatible APIs         | [libraries/scriptling/ai.md](libraries/scriptling/ai.md)           |
| `scriptling.ai.agent`   | Agentic AI loop with automatic tool execution           | [libraries/scriptling/agent.md](libraries/scriptling/agent.md)     |
| `scriptling.mcp`        | MCP (Model Context Protocol) tool interaction           | [libraries/scriptling/mcp.md](libraries/scriptling/mcp.md)         |
| `scriptling.toon`       | TOON (Token-Oriented Object Notation) encoding/decoding | [libraries/scriptling/toon.md](libraries/scriptling/toon.md)       |
| `scriptling.threads`    | Asynchronous execution with isolated environments       | [libraries/scriptling/threads.md](libraries/scriptling/threads.md) |
| `scriptling.console`    | Console input/output functions                          | [libraries/scriptling/console.md](libraries/scriptling/console.md) |

## Extended Libraries

These libraries provide Python-compatible functionality and require explicit registration by the host application (e.g., the CLI tool).

| Import        | Description                          | Details                                                            |
| ------------- | ------------------------------------ | ------------------------------------------------------------------ |
| `requests`    | HTTP library for sending requests    | [libraries/extlib/requests.md](libraries/extlib/requests.md)       |
| `sys`         | System-specific parameters           | [libraries/extlib/sys.md](libraries/extlib/sys.md)                 |
| `secrets`     | Cryptographically strong random nums | [libraries/extlib/secrets.md](libraries/extlib/secrets.md)         |
| `subprocess`  | Spawn and manage subprocesses        | [libraries/extlib/subprocess.md](libraries/extlib/subprocess.md)   |
| `html.parser` | HTML/XHTML parser                    | [libraries/extlib/html.parser.md](libraries/extlib/html.parser.md) |
| `os`          | Operating system interfaces          | [libraries/extlib/os.md](libraries/extlib/os.md)                   |
| `os.path`     | Pathname manipulations               | [libraries/extlib/os.path.md](libraries/extlib/os.path.md)         |
| `pathlib`     | Object-oriented filesystem paths     | [libraries/extlib/pathlib.md](libraries/extlib/pathlib.md)         |
| `glob`        | Unix shell-style wildcards           | [libraries/extlib/glob.md](libraries/extlib/glob.md)               |
| `logging`     | Logging functionality                | [libraries/extlib/logging.md](libraries/extlib/logging.md)         |
| `wait_for`    | Wait for resources to become avail   | [libraries/extlib/wait_for.md](libraries/extlib/wait_for.md)       |
| `yaml`        | YAML parsing and generation          | [libraries/extlib/yaml.md](libraries/extlib/yaml.md)               |

## Usage Example

```python
import json
import math
import logging

logger = logging.getLogger("myapp")
logger.info("Starting application")

data = json.loads('{"a": 1, "b": 2}')
print(math.sqrt(data["a"] + data["b"]))
```

For detailed documentation on each library, use the `help()` function within the interactive shell or script:

```python
import json
help(json)
import scriptling.ai as ai
help(ai)
```
