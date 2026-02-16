# Sandbox Library

The `scriptling.runtime.sandbox` sub-library provides isolated script execution environments. It allows a running script to create a fresh, independent Scriptling instance, set variables in it, execute code, and read variables back.

> **Note:** This is a sub-library of `scriptling.runtime`, registered automatically by `RegisterRuntimeLibraryAll()`. It is enabled by default in the Scriptling CLI and MCP server. A factory function must be configured via `SetSandboxFactory()` before `sandbox.create()` can be used.

## Import

```python
import scriptling.runtime as runtime

env = runtime.sandbox.create()
```

## Available Functions

| Function   | Description                                         |
| ---------- | --------------------------------------------------- |
| `create()` | Create a new isolated sandbox execution environment |

## Sandbox Object Methods

| Method              | Description                                          |
| ------------------- | ---------------------------------------------------- |
| `set(name, value)`  | Set a variable in the sandbox                        |
| `get(name)`         | Get a variable from the sandbox                      |
| `exec(code)`        | Execute script code in the sandbox                   |
| `exec_file(path)`   | Load and execute a script file in the sandbox        |
| `exit_code()`       | Get the exit code from the last execution            |

## Functions

### runtime.sandbox.create(capture_output=False)

Create a new isolated sandbox execution environment. The sandbox inherits the same library registrations and import paths as the parent but has its own completely independent variable scope.

By default, `print()` output from the sandbox is discarded. Set `capture_output=True` to send output to the parent's stdout instead.

**Parameters:**

- `capture_output` (bool, optional): If `True`, print output goes to stdout. Default: `False` (output discarded).

**Returns:** Sandbox object with `set()`, `get()`, `exec()`, `exec_file()`, and `exit_code()` methods.

```python
import scriptling.runtime as runtime

# Output from print() inside the sandbox is discarded
env = runtime.sandbox.create()

# Output from print() inside the sandbox goes to stdout
env = runtime.sandbox.create(capture_output=True)
```

## Sandbox Object

### env.set(name, value)

Set a variable in the sandbox's scope.

**Parameters:**

- `name` (str): Variable name
- `value` (any): Value to assign

```python
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("config", {"debug": True, "timeout": 30})
env.set("items", [1, 2, 3])
env.set("name", "test")
```

### env.get(name)

Retrieve a variable's value from the sandbox. Returns `None` if the variable does not exist.

**Parameters:**

- `name` (str): Variable name

**Returns:** The variable's value, or `None` if not found.

```python
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("x", 42)
env.exec("y = x * 2")
print(env.get("y"))  # 84
print(env.get("z"))  # None
```

### env.exec(code)

Execute script code within the sandbox. The code runs in the sandbox's isolated scope and can access variables set via `set()` and any imported libraries.

Errors during execution are captured internally — the script does not propagate errors to the caller. Check `exit_code()` after execution to determine success or failure.

If the script calls `sys.exit()` or a function that triggers a `SystemExit` (such as `scriptling.mcp.tool.return_string()`), the exit code is captured and available via `exit_code()`.

**Parameters:**

- `code` (str): Scriptling source code to execute

```python
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("data", [3, 1, 4, 1, 5])
env.exec("""
import json
data.sort()
result = json.dumps(data)
""")
print(env.get("result"))  # "[1, 1, 3, 4, 5]"
```

### env.exec_file(path)

Load a script from the filesystem and execute it within the sandbox. Equivalent to reading the file and calling `exec()` with its contents. Import resolution uses the same library paths as the parent.

Path restrictions configured via `SetSandboxAllowedPaths()` are enforced — files outside allowed directories will be rejected (exit_code set to 1).

**Parameters:**

- `path` (str): Path to the script file

```python
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("input_data", {"key": "value"})
env.exec_file("scripts/process.py")
output = env.get("result")
```

### env.exit_code()

Get the exit code from the last `exec()` or `exec_file()` call. Returns `0` for successful execution, non-zero for errors or explicit `sys.exit(N)` calls.

**Returns:** int - The exit code (0 = success).

```python
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.exec("x = 1 + 1")
print(env.exit_code())  # 0

env.exec("x = undefined_var")
print(env.exit_code())  # 1
```

## Usage Examples

### Running an Isolated Computation

```python
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("numbers", [10, 20, 30, 40, 50])
env.exec("""
total = 0
for n in numbers:
    total = total + n
average = total / len(numbers)
""")
print(env.get("average"))  # 30.0
```

### Executing a Script File with Parameters

```python
import scriptling.runtime as runtime

env = runtime.sandbox.create()
env.set("__mcp_params", {"customer_id": 123, "format": "json"})
env.exec_file("scripts/customer_report.py")

response = env.get("__mcp_response")
code = env.exit_code()
if code != 0:
    print("Script failed with exit code:", code)
else:
    print(response)
```

### Multiple Independent Sandboxes

```python
import scriptling.runtime as runtime

# Each sandbox has its own isolated scope
env1 = runtime.sandbox.create()
env2 = runtime.sandbox.create()

env1.set("x", 100)
env2.set("x", 200)

env1.exec("result = x * 2")
env2.exec("result = x * 3")

print(env1.get("result"))  # 200
print(env2.get("result"))  # 600
```

## Go Integration

### Registering the Library

The sandbox is a sub-library of `scriptling.runtime` and is registered automatically when using `RegisterRuntimeLibraryAll()`. Configure the factory and optional path restrictions before scripts call `sandbox.create()`.

```go
package main

import (
    "github.com/paularlott/scriptling"
    "github.com/paularlott/scriptling/extlibs"
    "github.com/paularlott/scriptling/stdlib"
)

func main() {
    p := scriptling.New()
    stdlib.RegisterAll(p)

    // Register runtime with all sub-libraries (http, kv, sync, sandbox)
    extlibs.RegisterRuntimeLibraryAll(p)

    // Configure the factory — each call creates a fully configured instance
    extlibs.SetSandboxFactory(func() extlibs.SandboxInstance {
        child := scriptling.New()
        stdlib.RegisterAll(child)
        extlibs.RegisterRuntimeLibraryAll(child)
        return child
    })

    // Optional: restrict exec_file() to specific directories
    extlibs.SetSandboxAllowedPaths([]string{"/opt/scripts"})

    p.Eval(`
import scriptling.runtime as runtime
env = runtime.sandbox.create()
env.set("x", 42)
env.exec("y = x + 1")
print(env.get("y"))
`)
}
```

### SandboxInstance Interface

The factory returns any type implementing `extlibs.SandboxInstance`:

```go
type SandboxInstance interface {
    SetObjectVar(name string, obj object.Object) error
    GetVarAsObject(name string) (object.Object, error)
    EvalWithContext(ctx context.Context, input string) (object.Object, error)
    SetSourceFile(name string)
    LoadLibraryIntoEnv(name string, env *object.Environment) error
    SetOutputWriter(w io.Writer)
}
```

`*scriptling.Scriptling` satisfies this interface. The same factory is used by both `runtime.sandbox` and `runtime.background()` — use `SetSandboxFactory()` to configure it for both.

### Path Restrictions

Use `SetSandboxAllowedPaths()` to restrict which files `exec_file()` can load:

```go
// Unrestricted — exec_file() can load any file (default)
extlibs.SetSandboxAllowedPaths(nil)

// Restricted — only allow scripts from specific directories
extlibs.SetSandboxAllowedPaths([]string{"/opt/scripts", "/home/user/scripts"})
```

Path restrictions use `fssecurity.Config` internally and protect against:
- Path traversal attacks (`../../../etc/passwd`)
- Symlink attacks (symlinks pointing outside allowed directories)
- Prefix attacks (`/allowed` vs `/allowed-other`)

### Factory Configuration

The factory function controls what each sandbox can do:

```go
// Full access — sandbox has all the same libraries as parent
extlibs.SetSandboxFactory(func() extlibs.SandboxInstance {
    child := scriptling.New()
    setupAllLibraries(child)
    return child
})

// Restricted — sandbox only gets safe libraries
extlibs.SetSandboxFactory(func() extlibs.SandboxInstance {
    child := scriptling.New()
    stdlib.RegisterAll(child)
    // No OS, subprocess, etc.
    return child
})
```

## Notes

- Each `sandbox.create()` call creates a **new Scriptling instance** via the configured factory. Sandboxes do not share state with each other or with the parent.
- By default, `print()` output from sandbox scripts is **discarded**. Use `capture_output=True` to see output.
- Errors in `exec()` and `exec_file()` are **captured internally**, not propagated to the caller. Always check `exit_code()` to detect failures.
- Libraries imported inside a sandbox are resolved through the same import paths configured by the factory (e.g., the `--libdir` flag in CLI mode).
- `SystemExit` exceptions (e.g., from `scriptling.mcp.tool.return_string()`) are captured and reported via `exit_code()` — they do not terminate the parent script.
- The factory must be configured before `sandbox.create()` is called. If no factory is set, `create()` returns an error.
- The same factory is used by both `scriptling.runtime.sandbox` and `scriptling.runtime.background()`. Call `extlibs.SetSandboxFactory()` to configure both at once.
- Path restrictions for `exec_file()` are configured via `SetSandboxAllowedPaths()`. By default, no restrictions are applied.
