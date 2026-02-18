# Security Guide

Scriptling provides a sandboxed Python-like execution environment, but proper security practices are essential when embedding it in your applications.

## Overview

Scriptling is designed with security in mind, but **you are responsible for configuring the sandbox appropriately** for your use case. The default configuration provides a balance between functionality and safety, but you should understand the security implications of your choices.

## Default Sandbox Behavior

By default, Scriptling:

- **Has NO access to the host file system** (unless explicitly granted)
- **Has NO network access** (unless explicitly enabled via `requests` library)
- **Runs in a memory-safe Go environment** (no C extensions)
- **Provides no direct access to Go's runtime or OS**
- **Has no access to environment variables** (unless explicitly provided)

## Library Security

### Safe Libraries (Standard Library)

These libraries are safe to use in most sandboxed environments and are available by default:

| Library       | Security Notes                               |
| ------------- | -------------------------------------------- |
| `math`        | Pure computation, no external access         |
| `json`        | Pure computation, no external access         |
| `datetime`    | Pure computation, no external access         |
| `time`        | Pure computation, no external access         |
| `string`      | Pure computation, no external access         |
| `base64`      | Pure computation, no external access         |
| `html`        | Pure computation, no external access         |
| `re`          | Regular expressions, no external access      |
| `random`      | Pseudo-random generation, no external access |
| `statistics`  | Pure computation, no external access         |
| `textwrap`    | Pure computation, no external access         |
| `functools`   | Pure computation, no external access         |
| `itertools`   | Pure computation, no external access         |
| `collections` | Pure computation, no external access         |
| `hashlib`     | Cryptographic hashing, no external access    |
| `platform`    | Platform information, read-only              |
| `urllib`      | URL parsing only, no network access          |
| `uuid`        | UUID generation, no external access          |

### Scriptling-Specific Libraries

These libraries provide Scriptling-specific functionality:

| Library               | Security Considerations                                       |
| --------------------- | ------------------------------------------------------------- |
| `scriptling.ai`       | **NETWORK ACCESS** - Makes HTTP requests to AI APIs           |
| `scriptling.ai.agent` | **NETWORK + CODE EXECUTION** - Agentic AI with tool execution |
| `scriptling.mcp`      | **NETWORK ACCESS** - MCP protocol communication               |
| `scriptling.console`  | Console I/O, safe for interactive use                         |
| `scriptling.fuzzy`    | Pure computation, no external access                          |
| `scriptling.toon`     | Pure computation, no external access                          |

### Runtime Libraries

These libraries provide server and concurrency functionality:

| Library                      | Security Considerations                                     |
| ---------------------------- | ----------------------------------------------------------- |
| `scriptling.runtime`         | Background task execution - safe but uses goroutines        |
| `scriptling.runtime.http`    | **HTTP SERVER** - Registers HTTP routes and handlers        |
| `scriptling.runtime.kv`      | In-memory key-value store - safe but unbounded memory       |
| `scriptling.runtime.sync`    | Concurrency primitives - safe but can cause deadlocks       |
| `scriptling.runtime.sandbox` | **CODE EXECUTION** - Executes code in isolated environments |

### Extended Libraries (Require Explicit Registration)

These libraries extend functionality but require explicit registration:

| Library       | Security Considerations                                                     |
| ------------- | --------------------------------------------------------------------------- |
| `requests`    | **NETWORK ACCESS** - Allows HTTP/HTTPS requests to external URLs            |
| `os`          | **FILE SYSTEM ACCESS** - Controlled by allowed paths                        |
| `os.path`     | **FILE SYSTEM ACCESS** - Path operations, controlled by allowed paths       |
| `pathlib`     | **FILE SYSTEM ACCESS** - Object-oriented paths, controlled by allowed paths |
| `glob`        | **FILE SYSTEM ACCESS** - Pattern matching, controlled by allowed paths      |
| `subprocess`  | **CRITICAL RISK** - Allows arbitrary command execution                      |
| `sys`         | **SYSTEM ACCESS** - Provides access to system internals                     |
| `secrets`     | Cryptographically secure random generation                                  |
| `logging`     | File and console logging - may write to disk                                |
| `wait_for`    | Network/resource polling - may access network                               |
| `yaml`        | YAML parsing - safe but watch for large files                               |
| `toml`        | TOML parsing - safe but watch for large files                               |
| `html.parser` | HTML parsing - safe but watch for large files                               |

### Never Register in Untrusted Environments

**Do NOT register these libraries when executing untrusted code:**

- `subprocess` - Allows arbitrary command execution
- `sys` - Provides access to system internals and environment
- `scriptling.runtime.sandbox` - Can execute arbitrary code
- `scriptling.ai.agent` - Can execute AI-generated code with tools

## File System Security

### Restricting File Access

When registering the `os`, `os.path`, `pathlib`, or `glob` libraries, you **must** specify allowed paths:

```go
// Safe: Only allows access to specific directories
extlibs.RegisterOSLibrary(p, []string{
    "/tmp/myapp/data",
    "/home/user/documents",
})
extlibs.RegisterPathlibLibrary(p, []string{
    "/tmp/myapp/data",
    "/home/user/documents",
})

// Dangerous: Allows access to entire file system
extlibs.RegisterOSLibrary(p, []string{}) // Empty = no restriction!
extlibs.RegisterOSLibrary(p, nil)       // Nil = read-only access everywhere
```

### Path Traversal Protection

Scriptling's file system libraries automatically prevent path traversal attacks:

```python
# User tries to escape allowed directory
import os
import pathlib

allowed_path = "/tmp/myapp/data"
# Trying to access parent directories
os.read_file("/tmp/myapp/data/../../etc/passwd")  # BLOCKED
os.read_file("/tmp/myapp/data/secrets.txt")        # ALLOWED

path = pathlib.Path("/tmp/myapp/data/../../../etc/passwd")  # BLOCKED
```

## Network Security

### Network-Enabled Libraries

These libraries can make network requests:

- `requests` - HTTP client library
- `scriptling.ai` - AI API client
- `scriptling.ai.agent` - Agentic AI with tool execution
- `scriptling.mcp` - MCP protocol client
- `wait_for` - Resource polling (may check network endpoints)

### Disabling Network Access

To keep the sandbox network-free, don't register network-enabled libraries:

```go
// Safe: Only register standard libraries (no network access)
stdlib.RegisterAll(p)

// Unsafe: Registers network-enabled library
extlibs.RegisterRequestsLibrary(p)
```

## Resource Limits

### Execution Timeout

Always set timeouts for script execution:

```go
import "time"

ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

result, err := p.EvalWithContext(ctx, code)
if err == context.DeadlineExceeded {
    // Script was terminated due to timeout
}
```

### Memory Limits

Scriptling runs within Go's memory management, but consider:

1. **Large allocations** can cause memory pressure
2. **Infinite loops** can consume CPU indefinitely
3. **Recursion depth** is limited by Go's stack

## Code Injection Prevention

### Never Execute Untrusted Input Directly

```python
# DANGEROUS: Never do this with scriptling.runtime.sandbox
import scriptling.runtime.sandbox as sandbox

user_input = get_user_code()  # e.g., "os.remove('/important/file')"
sandbox.eval(user_input)      # Executes arbitrary code!
```

**Note**: The `scriptling.runtime.sandbox` library allows executing arbitrary code strings. Never pass untrusted user input directly to it.

### Safe Patterns

```python
# SAFE: Use structured data and predefined functions
user_config = get_user_config()  # Returns validated dict
name = user_config.get("name", "Anonymous")
greet(name)  # Controlled execution

# SAFE: Whitelist allowed operations
allowed_operations = {"add", "subtract", "multiply"}
operation = user_config.get("operation")
if operation in allowed_operations:
    result = perform_operation(operation, x, y)
```

## Environment Variables

Environment variables are only accessible if the `sys` library is registered. Never register `sys` with untrusted code:

```go
// DANGEROUS: Allows access to all environment variables
extlibs.RegisterSysLibrary(p)

// SAFE: Don't register sys library for untrusted code
// Use SetVar to pass only necessary configuration
p.SetVar("APP_VERSION", "1.0.0")
p.SetVar("API_ENDPOINT", apiEndpoint)
```

## Security Checklist

Use this checklist when deploying Scriptling in production:

- [ ] File system access is restricted to specific paths (`os`, `pathlib`, `glob`)
- [ ] Network access is disabled or URL-filtered (`requests`, `scriptling.ai`, `scriptling.mcp`)
- [ ] Execution timeout is configured
- [ ] `subprocess` library is NOT registered
- [ ] `sys` library is NOT registered (or carefully controlled)
- [ ] `scriptling.runtime.sandbox` is NOT registered for untrusted code
- [ ] `scriptling.ai.agent` is NOT registered for untrusted code
- [ ] Environment variables are filtered
- [ ] Untrusted user input is validated
- [ ] Scripts run with minimal privileges
- [ ] Error messages don't leak sensitive information
- [ ] Logs are sanitized before display
- [ ] Runtime KV store size is monitored (unbounded memory)
- [ ] Background tasks are properly managed and terminated

## Common Attack Vectors

### 1. Resource Exhaustion

```python
# Consumes all memory
big_list = []
while True:
    big_list = big_list + ["x" * 1000000]
```

**Mitigation**: Use execution timeouts.

### 2. Infinite Loops

```python
# Consumes all CPU
while True:
    pass
```

**Mitigation**: Use execution timeouts.

### 3. Path Traversal (Protected)

```python
# Attempt to escape allowed directory
import os
os.read_file("../../etc/passwd")
```

**Mitigation**: Scriptling's `os` library validates paths against allowed directories.

### 4. Information Disclosure

```python
# Try to access internals
import sys
sys.get_environment_variables()  # Not available in default sandbox
```

**Mitigation**: Don't register `sys` or other introspection libraries.

## Best Practices

1. **Principle of Least Privilege**: Only register the libraries that are absolutely necessary
2. **Validate All Input**: Never trust user-provided code or data
3. **Use Timeouts**: Always set execution time limits
4. **Restrict File Access**: Explicitly whitelist allowed directories for `os`, `pathlib`, `glob`
5. **Disable Network**: Unless needed, keep the sandbox offline (don't register `requests`, `scriptling.ai`, `scriptling.mcp`)
6. **Monitor Resource Usage**: Watch for unusual memory/CPU consumption
7. **Sanitize Errors**: Don't expose internal paths or stack traces to users
8. **Keep Updated**: Update Scriptling regularly for security patches
9. **Limit Runtime State**: Monitor `scriptling.runtime.kv` store size and use TTLs
10. **Control Concurrency**: Be aware that `scriptling.runtime.sync` primitives can cause deadlocks
11. **Sandbox Isolation**: Use `scriptling.runtime.sandbox` carefully - it executes code in isolated environments but still runs in the same process

## Reporting Security Issues

If you discover a security vulnerability in Scriptling, please report it responsibly:

1. Do **NOT** create a public issue
2. Submit details via: [Security Report Form](https://forms.gle/86ckNpahQWPP5xRLA)
3. Include steps to reproduce
4. Allow time for a fix to be released before disclosure

## Additional Resources

- [Sandbox Configuration](GO_INTEGRATION.md)
- [Library Reference](LIBRARIES.md)
- [Error Handling](EXCEPTION_HANDLING.md)
