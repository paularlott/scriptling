# os Library (Extended)

The `os` library provides operating system interfaces for file system operations and environment variables. This is an **extended library** that must be explicitly registered.

> **Note:** This library requires security configuration. When using the Go API, you can specify allowed paths to restrict file system access for security.

## Import

```python
import os
```

## Security

The `os` library supports filesystem security restrictions. When registering the library, you can specify allowed paths:

```go
// Restrict to specific directories (recommended for untrusted scripts)
extlibs.RegisterOSLibrary(p, []string{"/tmp/sandbox", "/home/user/data"})

// No restrictions - full filesystem access (dangerous for untrusted code)
extlibs.RegisterOSLibrary(p, nil)
```

All file operations in the `os` library are restricted to the allowed directories. Path traversal attacks (`../../../etc/passwd`) and symlink attacks are prevented.

## Constants

### `os.sep`

The path separator used by the operating system.

```python
import os
print(os.sep)  # "/" on Unix, "\" on Windows
```

### `os.linesep`

The line separator used by the operating system.

```python
import os
print(os.linesep)  # "\n" on Unix, "\r\n" on Windows
```

### `os.name`

The operating system name.

```python
import os
print(os.name)  # "posix" on Unix/Linux/macOS, "nt" on Windows
```

## Functions

### os.getenv(key[, default])

Get an environment variable.

**Parameters:**
- `key` (string): Name of the environment variable
- `default` (optional): Value to return if the variable is not set

**Returns:** String value of the environment variable, or default if not set

```python
import os

# Get environment variable
home = os.getenv("HOME")
print(home)

# With default
path = os.getenv("MY_PATH", "/default/path")
print(path)
```

### os.environ()

Get all environment variables as a dictionary.

**Returns:** Dictionary of all environment variables

```python
import os

env = os.environ()
print(env["PATH"])
print(env["HOME"])
```

### os.getcwd()

Get the current working directory.

**Returns:** String path to the current working directory

```python
import os

cwd = os.getcwd()
print(cwd)  # e.g., "/home/user/projects"
```

### os.listdir(path=".")

List directory contents.

**Parameters:**
- `path` (string, optional): Directory path to list (default: current directory)

**Returns:** List of entry names in the directory

```python
import os

# List current directory
entries = os.listdir()
print(entries)  # ["file1.txt", "file2.py", "subdir"]

# List specific directory
entries = os.listdir("/tmp")
```

### os.read_file(path)

Read entire file contents as a string.

**Parameters:**
- `path` (string): Path to the file

**Returns:** String containing the file contents

```python
import os

content = os.read_file("/tmp/data.txt")
print(content)
```

### os.write_file(path, content)

Write content to a file (creates or overwrites).

**Parameters:**
- `path` (string): Path to the file
- `content` (string): Content to write

```python
import os

os.write_file("/tmp/output.txt", "Hello, World!")
```

### os.append_file(path, content)

Append content to a file.

**Parameters:**
- `path` (string): Path to the file
- `content` (string): Content to append

```python
import os

os.append_file("/tmp/log.txt", "New log entry\n")
```

### os.remove(path)

Remove a file.

**Parameters:**
- `path` (string): Path to the file to remove

```python
import os

os.remove("/tmp/old_file.txt")
```

### os.mkdir(path)

Create a directory.

**Parameters:**
- `path` (string): Path to the directory to create

```python
import os

os.mkdir("/tmp/newdir")
```

### os.makedirs(path)

Create directories recursively (creates all parent directories as needed).

**Parameters:**
- `path` (string): Path to the directory to create

```python
import os

os.makedirs("/tmp/a/b/c")  # Creates all directories in the path
```

### os.rmdir(path)

Remove an empty directory.

**Parameters:**
- `path` (string): Path to the directory to remove

```python
import os

os.rmdir("/tmp/emptydir")
```

### os.rename(old, new)

Rename a file or directory.

**Parameters:**
- `old` (string): Current path
- `new` (string): New path

```python
import os

os.rename("/tmp/old.txt", "/tmp/new.txt")
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

    // Register with path restrictions
    extlibs.RegisterOSLibrary(p, []string{"/tmp/sandbox", "/home/user/data"})

    // Or without restrictions (dangerous!)
    // extlibs.RegisterOSLibrary(p, nil)

    p.Eval(`
import os
print(os.getcwd())
    `)
}
```

## Examples

### Reading and Writing Files

```python
import os

# Write to a file
os.write_file("/tmp/data.txt", "Hello, World!")

# Read it back
content = os.read_file("/tmp/data.txt")
print(content)  # "Hello, World!"

# Append to it
os.append_file("/tmp/data.txt", "\nMore content")

# Clean up
os.remove("/tmp/data.txt")
```

### Working with Directories

```python
import os

# Create nested directories
os.makedirs("/tmp/myproject/src")
os.makedirs("/tmp/myproject/build")

# List contents
items = os.listdir("/tmp/myproject")
print(items)  # ["src", "build"]

# Clean up
os.remove("/tmp/myproject/src")
os.remove("/tmp/myproject/build")
os.rmdir("/tmp/myproject")
```

### Environment Variables

```python
import os

# Get specific environment variable
home = os.getenv("HOME", "/default/home")
print(f"Home directory: {home}")

# Get all environment variables
env = os.environ()
for key, value in env.items():
    print(f"{key} = {value}")
```

### Working with Different Path Separators

```python
import os

print(f"Path separator: {os.sep}")
print(f"Line separator: {repr(os.linesep)}")
print(f"OS name: {os.name}")

# Build paths correctly (though pathlib is preferred)
if os.name == "nt":
    path = "C:\\Users\\Documents"
else:
    path = "/home/user/documents"
```

## Python Compatibility

This library implements a subset of Python's `os` module:

| Function | Supported |
|----------|-----------|
| getenv | ✅ |
| environ | ✅ |
| getcwd | ✅ |
| listdir | ✅ |
| mkdir | ✅ |
| makedirs | ✅ |
| rmdir | ✅ |
| remove | ✅ |
| rename | ✅ |
| read_file | ✅ (Scriptling-specific) |
| write_file | ✅ (Scriptling-specific) |
| append_file | ✅ (Scriptling-specific) |
| stat | ❌ |
| walk | ❌ |
| chmod | ❌ |
| utime | ❌ |

## Differences from Python

- File operations use `read_file()`, `write_file()`, and `append_file()` instead of `open()`
- No file object handles - operations are direct functions
- All file operations are subject to security restrictions when configured
- No `os.path` module - use `import os.path` for path operations

## See Also

- [os.path](./os.path.md) - Path manipulation functions
- [pathlib](./pathlib.md) - Object-oriented filesystem paths
