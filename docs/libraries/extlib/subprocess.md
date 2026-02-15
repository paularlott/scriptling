# subprocess Library (Extended)

The `subprocess` library provides functions for spawning and managing subprocesses. This is an **extended library** that must be explicitly registered.

> **Note:** This library is enabled by default in the Scriptling CLI and MCP server but must be manually registered when using the Go API.

## Import

```python
import subprocess
```

## Available Functions

| Function                | Description                             |
| ----------------------- | --------------------------------------- |
| `run(args, options={})` | Run command and return CompletedProcess |

## Functions

### subprocess.run(args, options={})

Run a command and return a CompletedProcess instance.

**Parameters:**

- `args`: Command to run. Can be a string (split on spaces) or a list of arguments.
- `options`: Optional dictionary with configuration options.

**Options:**

- `capture_output` (boolean): Capture stdout and stderr. Default: `false`
- `shell` (boolean): Run command through shell. Default: `false`
- `cwd` (string): Working directory for command
- `timeout` (integer): Timeout in seconds
- `check` (boolean): Raise exception if returncode is non-zero. Default: `false`

**Returns:** CompletedProcess instance

**Examples:**

```python
import subprocess

# Simple command execution
result = subprocess.run("echo hello")
print(result.returncode)  # 0

# Capture output
result = subprocess.run("echo hello world", {"capture_output": true})
print(result.stdout.strip())  # "hello world"

# Run with arguments as list
result = subprocess.run(["echo", "test"])

# Use shell
result = subprocess.run("echo 'hello' && echo 'world'", {"shell": true})

# Check for errors
try:
    result = subprocess.run("false", {"check": true})
except:
    print("Command failed")
```

## CompletedProcess Class

The `subprocess.run()` function returns a `CompletedProcess` instance with the following attributes:

### Attributes

- `args`: List of command arguments
- `returncode`: Exit code of the process (integer)
- `stdout`: Captured standard output (string, empty if not captured)
- `stderr`: Captured standard error (string, empty if not captured)

### Methods

#### check_returncode()

Check if the process returned successfully. Raises an exception if returncode is non-zero.

**Returns:** The CompletedProcess instance if successful

**Example:**

```python
import subprocess

result = subprocess.run("true")
result.check_returncode()  # OK

try:
    result = subprocess.run("false")
    result.check_returncode()  # Raises exception
except:
    print("Process failed")
```

## Security Considerations

- Commands are executed in the host environment
- Be careful with user-provided command strings to avoid command injection
- The `shell` option can be dangerous with untrusted input
- Consider using argument lists instead of shell strings when possible

## Platform Compatibility

- Works on all platforms supported by Go's `os/exec` package
- Command syntax may vary between operating systems
- Use the `platform` library to detect the current platform if needed
