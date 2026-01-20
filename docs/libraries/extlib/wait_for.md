# wait_for Library

Functions for waiting on resources to become available. Useful for coordination between services, waiting for files, ports, or HTTP endpoints.

## Setup

The `wait_for` library is available by default in the scriptling-cli.

## Functions

### wait_for.file(path, timeout=30, poll_rate=1)

Waits for a file to exist.

**Parameters:**
- `path` (string): Path to the file to wait for
- `timeout` (int, optional): Maximum time to wait in seconds (default: 30)
- `poll_rate` (float, optional): Time between checks in seconds (default: 1)

**Returns:** `bool` - `True` if file exists, `False` if timeout exceeded

**Example:**
```python
import wait_for

# Wait for a file to be created (up to 2 minutes)
if wait_for.file("/mnt/nas/index_ready", timeout=120):
    print("File is ready!")
else:
    print("Timeout waiting for file")
```

### wait_for.dir(path, timeout=30, poll_rate=1)

Waits for a directory to exist.

**Parameters:**
- `path` (string): Path to the directory to wait for
- `timeout` (int, optional): Maximum time to wait in seconds (default: 30)
- `poll_rate` (float, optional): Time between checks in seconds (default: 1)

**Returns:** `bool` - `True` if directory exists, `False` if timeout exceeded

**Example:**
```python
import wait_for

# Wait for a backup directory to be created
if wait_for.dir("/mnt/nas/backups", timeout=60):
    print("Backup directory exists")
```

### wait_for.port(host, port, timeout=30, poll_rate=1)

Waits for a TCP port to accept connections.

**Parameters:**
- `host` (string): Hostname or IP address
- `port` (int|string): Port number
- `timeout` (int, optional): Maximum time to wait in seconds (default: 30)
- `poll_rate` (float, optional): Time between checks in seconds (default: 1)

**Returns:** `bool` - `True` if port is open, `False` if timeout exceeded

**Example:**
```python
import wait_for

# Wait for PostgreSQL to be ready
if wait_for.port("localhost", 5432, timeout=60):
    print("PostgreSQL is ready")
else:
    print("PostgreSQL did not start in time")
```

### wait_for.http(url, timeout=30, poll_rate=1, status_code=200)

Waits for an HTTP endpoint to respond with the expected status code.

**Parameters:**
- `url` (string): URL to check
- `timeout` (int, optional): Maximum time to wait in seconds (default: 30)
- `poll_rate` (float, optional): Time between checks in seconds (default: 1)
- `status_code` (int, optional): Expected HTTP status code (default: 200)

**Returns:** `bool` - `True` if endpoint responds with expected status, `False` if timeout exceeded

**Example:**
```python
import wait_for

# Wait for a health check endpoint
if wait_for.http("http://localhost:8080/health", timeout=60):
    print("Service is healthy")

# Wait for a specific status code
if wait_for.http("http://api.example.com/ready", timeout=30, status_code=200):
    print("API is ready")
```

### wait_for.file_content(path, content, timeout=30, poll_rate=1)

Waits for a file to exist and contain specific content.

**Parameters:**
- `path` (string): Path to the file to check
- `content` (string): Content to search for in the file
- `timeout` (int, optional): Maximum time to wait in seconds (default: 30)
- `poll_rate` (float, optional): Time between checks in seconds (default: 1)

**Returns:** `bool` - `True` if file contains the content, `False` if timeout exceeded

**Example:**
```python
import wait_for

# Wait for a readiness file to contain "ready=true"
if wait_for.file_content("/tmp/service.status", "ready=true", timeout=120):
    print("Service is ready")

# Wait for a specific pattern in a log file
if wait_for.file_content("/var/log/app.log", "Server started", timeout=30):
    print("Application has started")
```

### wait_for.process_name(name, timeout=30, poll_rate=1)

Waits for a process with the specified name to be running.

**Parameters:**
- `name` (string): Process name to search for
- `timeout` (int, optional): Maximum time to wait in seconds (default: 30)
- `poll_rate` (float, optional): Time between checks in seconds (default: 1)

**Returns:** `bool` - `True` if process is running, `False` if timeout exceeded

**Example:**
```python
import wait_for

# Wait for nginx to start
if wait_for.process_name("nginx", timeout=30):
    print("nginx is running")

# Wait for a custom application
if wait_for.process_name("my-app", timeout=60):
    print("Application is running")
```

## Complete Examples

### Database Migration Coordination

```python
import wait_for

# Wait for PostgreSQL to be ready
if not wait_for.port("localhost", 5432, timeout=60):
    print("ERROR: PostgreSQL did not start")
    exit(1)

# Run migrations
import subprocess
subprocess.run(["python", "manage.py", "migrate"])

# Wait for the migration lock file to be removed
while wait_for.file("/tmp/migration.lock", timeout=1):
    print("Waiting for migrations to complete...")

print("Migrations complete, starting application")
```

### Multi-Service Startup Coordination

```python
import wait_for
import time

# Start Redis in background
import subprocess
subprocess.Popen(["redis-server"])

# Start PostgreSQL in background
subprocess.Popen(["postgres", "-D", "/usr/local/var/postgres"])

# Wait for both services
print("Waiting for services...")
if wait_for.port("localhost", 6379, timeout=30):
    print("  ✓ Redis is ready")
else:
    print("  ✗ Redis failed to start")
    exit(1)

if wait_for.port("localhost", 5432, timeout=30):
    print("  ✓ PostgreSQL is ready")
else:
    print("  ✗ PostgreSQL failed to start")
    exit(1)

print("All services are ready!")
```

### HTTP Service Health Check

```python
import wait_for

services = [
    ("http://localhost:8080/health", "API Gateway"),
    ("http://localhost:8081/health", "Auth Service"),
    ("http://localhost:8082/health", "Data Service"),
]

print("Waiting for services to be healthy...")
for url, name in services:
    if wait_for.http(url, timeout=60):
        print(f"  ✓ {name}")
    else:
        print(f"  ✗ {name} failed health check")
        exit(1)

print("All services are healthy!")
```

### Waiting for File Changes

```python
import wait_for

# Wait for an index build to complete
# The index_ready file is created when indexing starts
# and contains "complete" when done

print("Waiting for index build...")
if wait_for.file("/data/index_ready", timeout=300):
    print("Index build started...")
    if wait_for.file_content("/data/index_ready", "complete", timeout=600):
        print("Index build complete!")
    else:
        print("Index build timed out")
else:
    print("Index build never started")
```

### Custom Poll Rate

```python
import wait_for

# Poll more frequently for quick checks (every 0.5 seconds)
if wait_for.port("localhost", 8080, timeout=10, poll_rate=0.5):
    print("Port opened quickly")

# Poll less frequently for long waits (every 5 seconds)
if wait_for.file("/mnt/nas/large_file.zip", timeout=600, poll_rate=5):
    print("File is ready")
```

## Notes

- All functions return `True` if the resource became available, `False` if timeout was exceeded
- The default timeout is 30 seconds for all functions
- The default poll rate is 1 second (checks once per second)
- For HTTP checks, the default expected status code is 200
- Process name matching is platform-dependent (uses `/proc` on Linux, `ps` on macOS/BSD)
- Port checking uses TCP connection attempts with a 1-second timeout per attempt
