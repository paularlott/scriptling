# os.path Library (Extended)

The `os.path` library provides common pathname manipulations. This is an **extended library** that must be explicitly registered (registered together with the `os` library).

> **Note:** This library is registered automatically when you register the `os` library using `RegisterOSLibrary()`.

## Import

```python
import os.path
```

## Available Functions

| Function               | Description                       |
| ---------------------- | --------------------------------- |
| `join(*paths)`         | Join path components              |
| `exists(path)`         | Check if path exists              |
| `isfile(path)`         | Check if path is a file           |
| `isdir(path)`          | Check if path is a directory      |
| `basename(path)`       | Get final component of path       |
| `dirname(path)`        | Get directory component of path   |
| `split(path)`          | Split path into (head, tail)      |
| `splitext(path)`       | Split path into (root, extension) |
| `normpath(path)`       | Normalize path (remove . and ..)  |
| `abspath(path)`        | Get absolute path                 |
| `relpath(path, start)` | Get relative path from start      |
| `commonprefix(list)`   | Get longest common path prefix    |

## Security

Like the `os` library, `os.path` operations are subject to filesystem security restrictions when configured. All path operations are restricted to the allowed directories.

## Functions

### os.path.join(\*paths)

Join path components using the appropriate separator for the OS.

**Parameters:**

- `*paths`: Path components to join

**Returns:** Joined path string

```python
import os.path

# Join path components
path = os.path.join("home", "user", "documents")
print(path)  # "home/user/documents" on Unix, "home\user\documents" on Windows

# Multiple components
full = os.path.join("/", "home", "user", "documents", "file.txt")
print(full)  # "/home/user/documents/file.txt"
```

### os.path.exists(path)

Check if a path exists.

**Parameters:**

- `path` (string): Path to check

**Returns:** Boolean - `True` if the path exists, `False` otherwise

```python
import os.path

if os.path.exists("/tmp/file.txt"):
    print("File exists")
else:
    print("File does not exist")
```

### os.path.isfile(path)

Check if a path is a regular file.

**Parameters:**

- `path` (string): Path to check

**Returns:** Boolean - `True` if the path is a file, `False` otherwise

```python
import os.path

if os.path.isfile("/tmp/data.txt"):
    print("It's a file")
```

### os.path.isdir(path)

Check if a path is a directory.

**Parameters:**

- `path` (string): Path to check

**Returns:** Boolean - `True` if the path is a directory, `False` otherwise

```python
import os.path

if os.path.isdir("/tmp/mydir"):
    print("It's a directory")
```

### os.path.basename(path)

Get the base name of a path (final component).

**Parameters:**

- `path` (string): Path to process

**Returns:** String - the final component of the path

```python
import os.path

print(os.path.basename("/home/user/file.txt"))  # "file.txt"
print(os.path.basename("/home/user/mydir/"))    # "mydir"
```

### os.path.dirname(path)

Get the directory name of a path.

**Parameters:**

- `path` (string): Path to process

**Returns:** String - the directory component of the path

```python
import os.path

print(os.path.dirname("/home/user/file.txt"))  # "/home/user"
print(os.path.dirname("/home/user/mydir/"))    # "/home/user"
```

### os.path.split(path)

Split a path into (directory, filename) tuple.

**Parameters:**

- `path` (string): Path to split

**Returns:** Tuple of (directory, filename)

```python
import os.path

dir, file = os.path.split("/home/user/file.txt")
print(dir)   # "/home/user"
print(file)  # "file.txt"
```

### os.path.splitext(path)

Split a path into (root, extension) tuple.

**Parameters:**

- `path` (string): Path to split

**Returns:** Tuple of (root, extension)

```python
import os.path

root, ext = os.path.splitext("/home/user/file.txt")
print(root)  # "/home/user/file"
print(ext)   # ".txt"

root, ext = os.path.splitext("/home/user/archive.tar.gz")
print(root)  # "/home/user/archive.tar"
print(ext)   # ".gz"
```

### os.path.abspath(path)

Get the absolute path.

**Parameters:**

- `path` (string): Path to convert

**Returns:** String - absolute path

```python
import os.path

print(os.path.abspath("file.txt"))          # e.g., "/home/user/project/file.txt"
print(os.path.abspath("../other/file.txt"))  # e.g., "/home/user/other/file.txt"
```

### os.path.normpath(path)

Normalize a path by collapsing redundant separators and up-level references.

**Parameters:**

- `path` (string): Path to normalize

**Returns:** String - normalized path

```python
import os.path

print(os.path.normpath("home//user/../user/./docs"))
# "home/user/docs"

print(os.path.normpath("/a/b/c/../../d"))
# "/a/d"
```

### os.path.relpath(path[, start])

Get a relative path to a file.

**Parameters:**

- `path` (string): Path to convert to relative
- `start` (string, optional): Starting directory (default: current directory)

**Returns:** String - relative path

```python
import os.path

# From current directory
print(os.path.relpath("/home/user/project/file.txt"))
# e.g., "file.txt" if cwd is /home/user/project

# From specific start directory
print(os.path.relpath("/home/user/project/file.txt", "/home/user"))
# "project/file.txt"
```

### os.path.isabs(path)

Check if a path is absolute.

**Parameters:**

- `path` (string): Path to check

**Returns:** Boolean - `True` if the path is absolute, `False` otherwise

```python
import os.path

print(os.path.isabs("/home/user/file.txt"))  # True
print(os.path.isabs("file.txt"))              # False
print(os.path.isabs("../parent/file.txt"))    # False
```

### os.path.getsize(path)

Get the size of a file in bytes.

**Parameters:**

- `path` (string): Path to the file

**Returns:** Integer - file size in bytes

```python
import os.path

size = os.path.getsize("/tmp/data.txt")
print(f"File size: {size} bytes")
```

### os.path.getmtime(path)

Get the time of last modification of a file.

**Parameters:**

- `path` (string): Path to the file

**Returns:** Float - modification time as Unix timestamp (seconds since epoch)

```python
import os.path

mtime = os.path.getmtime("/tmp/data.txt")
print(f"Last modified: {mtime}")

# Convert to readable format
import time
readable = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(mtime))
print(f"Last modified: {readable}")
```

## Enabling in Go

```go
package main

import (
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/extlibs"
)

func main() {
    p := scriptling.New()

    // Register os.path (registered together with os)
    extlibs.RegisterOSLibrary(p, []string{"/tmp/sandbox"})

    p.Eval(`
import os.path
path = os.path.join("home", "user", "documents")
print(path)
    `)
}
```

## Examples

### Building Platform-Independent Paths

```python
import os.path

# Build paths that work on any OS
config_dir = os.path.join("config", "settings")
data_file = os.path.join(config_dir, "data.json")

print(data_file)  # "config/settings/data.json" (Unix) or "config\settings\data.json" (Windows)
```

### Parsing Paths

```python
import os.path

full_path = "/home/user/documents/report.pdf"

# Get components
directory = os.path.dirname(full_path)
filename = os.path.basename(full_path)

print(f"Directory: {directory}")  # "/home/user/documents"
print(f"Filename: {filename}")    # "report.pdf"

# Split extension
root, ext = os.path.splitext(filename)
print(f"Root: {root}, Extension: {ext}")  # "report", ".pdf"
```

### Checking File Types

```python
import os.path

path = "/tmp/myfile"

if os.path.exists(path):
    if os.path.isfile(path):
        print(f"{path} is a file")
        size = os.path.getsize(path)
        print(f"Size: {size} bytes")
    elif os.path.isdir(path):
        print(f"{path} is a directory")
else:
    print(f"{path} does not exist")
```

### Path Normalization

```python
import os.path

# Clean up messy paths
messy = "home//user/../user/./docs/../data"
clean = os.path.normpath(messy)
print(clean)  # "home/user/data"

# Convert to absolute path
abs_path = os.path.abspath(clean)
print(abs_path)  # e.g., "/home/user/project/home/user/data"
```

### Relative Path Calculation

```python
import os.path

# Find relative path from one location to another
source = "/home/user/project/src/main.py"
target = "/home/user/project/config/settings.json"

# Get relative path from source directory to target
source_dir = os.path.dirname(source)
relative = os.path.relpath(target, source_dir)
print(relative)  # "../config/settings.json"
```

## Python Compatibility

This library implements a subset of Python's `os.path` module:

| Function | Supported |
| -------- | --------- |
| join     | ✅        |
| exists   | ✅        |
| isfile   | ✅        |
| isdir    | ✅        |
| basename | ✅        |
| dirname  | ✅        |
| split    | ✅        |
| splitext | ✅        |
| abspath  | ✅        |
| normpath | ✅        |
| relpath  | ✅        |
| isabs    | ✅        |
| getsize  | ✅        |
| getmtime | ✅        |
| getatime | ❌        |
| getctime | ❌        |
| islink   | ❌        |
| ismount  | ❌        |
| samefile | ❌        |

## See Also

- [os](./os.md) - Operating system interfaces and file operations
- [pathlib](./pathlib.md) - Object-oriented filesystem paths (recommended for new code)
