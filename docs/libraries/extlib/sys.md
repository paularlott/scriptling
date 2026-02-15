# sys Library (Extended)

The `sys` library provides access to system-specific parameters and functions. This is an **extended library** that must be explicitly registered.

> **Note:** This library is enabled by default in the Scriptling CLI but must be manually registered when using the Go API.

## Import

```python
import sys
```

## Available Constants

| Constant   | Description                      |
| ---------- | -------------------------------- |
| `platform` | Operating system platform string |
| `version`  | Scriptling interpreter version   |
| `maxsize`  | Maximum signed integer value     |
| `path_sep` | Path separator for OS            |
| `argv`     | List of command-line arguments   |

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

Raise a SystemExit exception to exit the interpreter.

**Parameters:**

- `code` - Exit status (default: 0). Can be an integer or a string message.

**Behavior:**

- Raises a `SystemExit` exception that can be caught with try/except
- If not caught, terminates the script execution with an error
- Integer codes produce exception message: "SystemExit: <code>"
- String messages produce exception with that message directly

**Examples:**

```python
import sys

# Exit successfully (raises SystemExit: 0)
sys.exit()

# Exit with error code (raises SystemExit: 1)
sys.exit(1)

# Exit with custom message
sys.exit("Fatal error occurred")

# Catching sys.exit to prevent termination
try:
    sys.exit(42)
    print("This won't run")
except Exception as e:
    print("Caught:", str(e))  # "Caught: SystemExit: 42"
    # Continue execution instead of exiting
```

**Note:** SystemExit exceptions have an `ExceptionType` of "SystemExit" and can be detected by the caller via the returned error.

## Go API Integration

When using the Go API, `sys.exit()` returns a `*extlibs.SysExitCode` error that contains the exit code. The caller can decide how to handle it:

```go
package main

import (
    "fmt"
    "os"
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/extlibs"
)

func main() {
    p := scriptling.New()

    // Register the sys library with argv
    extlibs.RegisterSysLibrary(p, []string{"script.py", "arg1", "arg2"})

    // Evaluate code that calls sys.exit()
    result, err := p.Eval(`
import sys
sys.exit(42)
    `)

    // Check for SystemExit error using helper
    if sysExit, ok := extlibs.GetSysExitCode(err); ok {
        fmt.Printf("Script exited with code: %d\n", sysExit.Code)
        os.Exit(sysExit.Code)  // Optionally exit the Go process
    } else if err != nil {
        fmt.Printf("Error: %v\n", err)
        os.Exit(1)
    }
}
```

**Helper functions:**

- `extlibs.IsSysExitCode(err error) bool` - Check if an error is a SysExitCode
- `extlibs.GetSysExitCode(err error) (*SysExitCode, bool)` - Extract the SysExitCode from an error

**Key change from previous behavior:** `sys.exit()` no longer terminates the Go process directly. Instead, it returns an error that the caller can handle. This prevents scripts from unexpectedly terminating the host application.

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

| Feature             | Supported       |
| ------------------- | --------------- |
| argv                | ✅              |
| exit()              | ✅              |
| platform            | ✅              |
| version             | ✅ (simplified) |
| maxsize             | ✅              |
| path                | ❌              |
| modules             | ❌              |
| stdin/stdout/stderr | ❌              |
| executable          | ❌              |
| version_info        | ❌              |
