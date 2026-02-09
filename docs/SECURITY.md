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

### Safe Libraries (Default)

These libraries are safe to use in most sandboxed environments:

| Library | Security Notes |
|---------|---------------|
| `math` | Pure computation, no external access |
| `json` | Pure computation, no external access |
| `datetime` | Pure computation, no external access |
| `string` | Pure computation, no external access |
| `list` | Pure computation, no external access |
| `dict` | Pure computation, no external access |

### Extended Libraries (Require Explicit Registration)

These libraries extend functionality but require explicit registration:

| Library | Security Considerations |
|---------|------------------------|
| `requests` | Allows network HTTP/HTTPS requests to external URLs |
| `os` | Provides controlled file system access based on allowed paths |
| `pathlib` | File path manipulation (restricted to allowed paths) |
| `subprocess` | **HIGH RISK** - Allows executing external commands |
| `secrets` | Secure random generation, but needs proper seeding |
| `threads` | **HIGH RISK** - Can cause resource exhaustion through goroutine spawning |

### Never Register in Production

**Do NOT register these libraries in untrusted environments:**

- `subprocess` - Allows arbitrary command execution
- `threads` - Can cause resource exhaustion and denial of service
- `sys` - Provides access to system internals

## File System Security

### Restricting File Access

When registering the `os` or `pathlib` libraries, you **must** specify allowed paths:

```go
// Safe: Only allows access to specific directories
extlibs.RegisterOSLibrary(p, []string{
    "/tmp/myapp/data",
    "/home/user/documents",
})

// Dangerous: Allows access to entire file system
extlibs.RegisterOSLibrary(p, []string{}) // Empty = no restriction!
extlibs.RegisterOSLibrary(p, nil)       // Nil = read-only access everywhere
```

### Path Traversal Protection

Scriptling's `os` library automatically prevents path traversal attacks:

```python
# User tries to escape allowed directory
import os

allowed_path = "/tmp/myapp/data"
# Trying to access parent directories
os.read_file("/tmp/myapp/data/../../etc/passwd")  # BLOCKED
os.read_file("/tmp/myapp/data/secrets.txt")        # ALLOWED
```

## Network Security

### HTTP Library Configuration

The `requests` library supports network access. To disable it:

```go
// Don't register the requests library to keep sandbox network-free
stdlib.RegisterAll(p)  // Does NOT include requests by default
```

### URL Whitelisting (Recommended)

For production, consider implementing URL filtering before registration:

```go
// Example: Custom requests wrapper with URL filtering
func registerSafeRequests(p *scriptling.Program) {
    // You would need to implement this wrapper
    extlibs.RegisterFilteredRequestsLibrary(p, allowedDomains)
}
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
4. **Goroutine spawning** (via `threads` library) can exhaust resources if not properly limited

### Thread Limits (If Using threads Library)

If you must register the `threads` library:

```go
// Implement a goroutine pool with limits
type ThreadPool struct {
    maxGoRoutines int
    semaphore     chan struct{}
}

func (p *ThreadPool) Acquire() error {
    select {
    case p.semaphore <- struct{}{}:
        return nil
    default:
        return fmt.Errorf("maximum concurrent goroutines reached")
    }
}

func (p *ThreadPool) Release() {
    <-p.semaphore
}
```

**Risks of unbounded threading:**
- Goroutine exhaustion can crash the host process
- Each goroutine consumes stack memory (~2KB minimum)
- Unbounded concurrency can cause scheduler thrashing

## Code Injection Prevention

### Never Execute Untrusted Input Directly

```python
# DANGEROUS: Never do this
user_input = get_user_code()  # e.g., "os.remove('/important/file')"
eval(user_input)              # Executes arbitrary code!
```

### Safe Patterns

```python
# SAFE: Use structured data
user_config = get_user_config()  # Returns validated dict
name = user_config.get("name", "Anonymous")
greet(name)  # Controlled execution
```

## Environment Variables

Never expose sensitive environment variables to scripts:

```go
// DANGEROUS: Exposes all environment variables
// p.SetEnv(os.Environ())

// SAFE: Only expose necessary variables
p.SetEnv(map[string]string{
    "APP_VERSION": "1.0.0",
    "API_ENDPOINT": apiEndpoint, // User-provided, but controlled
})
```

## Security Checklist

Use this checklist when deploying Scriptling in production:

- [ ] File system access is restricted to specific paths
- [ ] Network access is disabled or URL-filtered
- [ ] Execution timeout is configured
- [ ] `subprocess` library is NOT registered
- [ ] `threads` library is NOT registered (or properly limited)
- [ ] Environment variables are filtered
- [ ] Untrusted user input is validated
- [ ] Scripts run with minimal privileges
- [ ] Error messages don't leak sensitive information
- [ ] Logs are sanitized before display

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

### 5. Thread-based Resource Exhaustion (If threads Library is Registered)

```python
# Spawn unlimited goroutines to exhaust resources
import threads

while True:
    threads.spawn(lambda: while True: pass)  # Each spawns a CPU-eating goroutine
```

**Mitigation**: Do NOT register `threads` library in untrusted environments. If required, implement a goroutine pool with strict limits.

## Best Practices

1. **Principle of Least Privilege**: Only register the libraries that are absolutely necessary
2. **Validate All Input**: Never trust user-provided code or data
3. **Use Timeouts**: Always set execution time limits
4. **Restrict File Access**: Explicitly whitelist allowed directories
5. **Disable Network**: Unless needed, keep the sandbox offline
6. **Monitor Resource Usage**: Watch for unusual memory/CPU consumption
7. **Sanitize Errors**: Don't expose internal paths or stack traces to users
8. **Keep Updated**: Update Scriptling regularly for security patches

## Reporting Security Issues

If you discover a security vulnerability in Scriptling, please report it responsibly:

1. Do **NOT** create a public issue
2. Email details to: security@example.com
3. Include steps to reproduce
4. Allow time for a fix to be released before disclosure

## Additional Resources

- [Sandbox Configuration](GO_INTEGRATION.md)
- [Library Reference](LIBRARIES.md)
- [Error Handling](EXCEPTION_HANDLING.md)
