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
- `append(list, item)` - Append item (modifies list in-place)
- `extend(list, other_list)` - Append elements from another list (modifies list in-place)
- `pop(list, index)` - Remove and return item at index
- `insert(list, index, item)` - Insert item at index
- `remove(list, item)` - Remove first occurrence of item
- `index(list, item)` - Return index of item
- `count(list, item)` - Count occurrences of item
- `sort(list)` - Sort list in-place
- `reverse(list)` - Reverse list in-place

### Dictionary Functions
- `keys(dict)` - Get list of keys
- `values(dict)` - Get list of values
- `items(dict)` - Get list of (key, value) tuples
- `get(dict, key, default)` - Get value with default
- `pop(dict, key, default)` - Remove and return value
- `update(dict, other)` - Update with other dictionary

### System
- `import library_name` - Load library dynamically
- `help([object])` - Display help information for functions, libraries, and objects
- `type(object)` - Get type of object
- `isinstance(object, type)` - Check if object is instance of type
- `dir(object)` - List attributes of object

## Standard Libraries

These libraries are built-in and can be imported.

### Data Handling
- **`json`**: Parse and generate JSON data.
- **`base64`**: Base64 encoding and decoding.
- **`html`**: HTML escaping and unescaping.

### Math & Numbers
- **`math`**: Mathematical functions and constants (`sin`, `cos`, `sqrt`, `pi`, etc.).
- **`random`**: Random number generation (`random`, `randint`, `choice`, `shuffle`).
- **`statistics`**: Statistical functions (`mean`, `median`, `mode`, `stdev`).

### Date & Time
- **`time`**: Time access and conversions.
- **`datetime`**: Basic date and time types.

### Text Processing
- **`re`**: Regular expression operations.
- **`string`**: Common string operations and constants.
- **`textwrap`**: Text wrapping and filling.

### Functional Programming
- **`functools`**: Higher-order functions and operations on callable objects (`reduce`, `partial`).
- **`itertools`**: Functions creating iterators for efficient looping (`count`, `cycle`, `repeat`).

### Collections & Algorithms
- **`collections`**: Container datatypes (`deque`, `Counter`, `defaultdict`).
- **`copy`**: Shallow and deep copy operations.
- **`hashlib`**: Secure hash and message digest algorithms (`md5`, `sha256`).

### System & Network
- **`platform`**: Access to underlying platform's identifying data.
- **`urllib`**: URL handling modules (`urllib.parse`, `urllib.request`).
- **`uuid`**: UUID generation.
- **`requests`**: HTTP library for sending requests.

## Extended Libraries (Host Registration Required)

These libraries are not loaded by default and must be explicitly registered by the host application (e.g., the CLI tool).

- **`os`**: Operating system interfaces (filesystem access). Requires security configuration.
- **`pathlib`**: Object-oriented filesystem paths.
- **`secrets`**: Cryptographically strong random numbers.
- **`subprocess`**: Spawn and manage subprocesses.
- **`sys`**: System-specific parameters and functions (`argv`, `exit`, `version`).

## External / System Libraries

These libraries provide access to system resources and may be restricted in sandboxed environments.

- **`os`**: Operating system interfaces (file system access).
- **`pathlib`**: Object-oriented filesystem paths.
- **`sys`**: System-specific parameters and functions.
- **`requests`**: HTTP library for human beings.
- **`secrets`**: Generate secure random numbers for managing secrets.

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
