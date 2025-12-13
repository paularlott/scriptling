# sys Library (Extended)

The `sys` library provides access to system-specific parameters and functions. This is an **extended library** that must be explicitly registered.

> **Note:** This library is enabled by default in the Scriptling CLI but must be manually registered when using the Go API.

## Import

```python
import sys
```

## Constants

### platform
A string identifying the operating system platform.

```python
import sys
print(sys.platform)  # "darwin", "linux", or "win32"
```

### version
A string containing the version of the Scriptling interpreter.

```python
import sys
print(sys.version)  # "Scriptling 1.0"
```

### maxsize
The maximum value of a signed integer (int64).

```python
import sys
print(sys.maxsize)  # 9223372036854775807
```

### path_sep
The path separator used by the operating system.

```python
import sys
print(sys.path_sep)  # "/" on Unix, "\" on Windows
```

### argv
A list of command-line arguments passed to the script.

```python
import sys
print(sys.argv)  # ["script.py", "arg1", "arg2"]
```

## Functions

### exit([code])
Exit the interpreter with an optional status code.

**Parameters:**
- `code` - Exit status (default: 0). Can be an integer or a string message.

**Examples:**

```python
import sys

# Exit successfully
sys.exit()

# Exit with error code
sys.exit(1)

# Exit with error message
sys.exit("Fatal error occurred")
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

    // Register the sys library with argv
    p.RegisterLibrary("sys", extlibs.NewSysLibrary([]string{"script.py", "arg1", "arg2"}))

    // Optionally set up exit callback
    extlibs.SysExitCallback = func(code int) {
        os.Exit(code)
    }

    p.Eval(`
import sys
print(sys.argv)
    `)
}
```

## Examples

### Check Platform
```python
import sys

if sys.platform == "darwin":
    print("Running on macOS")
elif sys.platform == "linux":
    print("Running on Linux")
elif sys.platform == "win32":
    print("Running on Windows")
```

### Process Arguments
```python
import sys

if len(sys.argv) < 2:
    print("Usage: script.py <input_file>")
    sys.exit(1)

input_file = sys.argv[1]
print(f"Processing {input_file}")
```

## Python Compatibility

This library implements a subset of Python's `sys` module:

| Feature | Supported |
|---------|-----------|
| argv | ✅ |
| exit() | ✅ |
| platform | ✅ |
| version | ✅ (simplified) |
| maxsize | ✅ |
| path | ❌ |
| modules | ❌ |
| stdin/stdout/stderr | ❌ |
| executable | ❌ |
| version_info | ❌ |
