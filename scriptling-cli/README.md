# Scriptling CLI

A command-line interface for the Scriptling programming language.

## Installation

Download the appropriate binary for your platform from the releases or build from source.

## Usage

### Run a script file

```bash
scriptling script.py
```

### Run from stdin

```bash
echo 'print("Hello World")' | scriptling
```

### Interactive mode

```bash
scriptling --interactive
# or
scriptling -i
```

### Lint mode

Lint scripts for syntax errors without executing them:

```bash
# Lint a file
scriptling --lint script.py

# Lint from stdin
echo 'x = 1 +' | scriptling --lint

# JSON output format
scriptling --lint --lint-format json script.py
```

**Output format (text):**
```
script.py:3: expected token COLON (error)
```

**Output format (JSON):**
```json
{
  "files_checked": 1,
  "has_errors": true,
  "errors": [
    {
      "file": "script.py",
      "line": 3,
      "message": "expected token COLON",
      "severity": "error",
      "code": "parse-error"
    }
  ]
}
```

The linter exits with code 0 if no errors are found, and code 1 if any errors exist.

### Command Line Options

| Flag | Environment Variable | Description | Default |
|------|---------------------|-------------|---------|
| `-i`, `--interactive` | - | Start interactive mode | false |
| `-l`, `--lint` | - | Lint script files without executing | false |
| `--lint-format` | `SCRIPTLING_LINT_FORMAT` | Output format for lint (text/json) | text |
| `--libdir` | `SCRIPTLING_LIBDIR` | Directory to load libraries from | (current dir) |
| `--log-level` | `SCRIPTLING_LOG_LEVEL` | Log level (trace/debug/info/warn/error) | info |
| `--log-format` | `SCRIPTLING_LOG_FORMAT` | Log format (console/json) | console |
| `-S`, `--server` | `SCRIPTLING_SERVER` | HTTP server address (host:port) | (disabled) |
| `--mcp-tools` | `SCRIPTLING_MCP_TOOLS` | Directory containing MCP tools | (disabled) |
| `--bearer-token` | `SCRIPTLING_BEARER_TOKEN` | Bearer token for authentication | none |
| `--script-mode` | `SCRIPTLING_SCRIPT_MODE` | Script mode: safe or full | full |
| `--allowed-paths` | `SCRIPTLING_ALLOWED_PATHS` | Comma-separated allowed filesystem paths | (no restriction) |
| `--tls-cert` | `SCRIPTLING_TLS_CERT` | TLS certificate file | none |
| `--tls-key` | `SCRIPTLING_TLS_KEY` | TLS key file | none |
| `--tls-generate` | - | Generate self-signed certificate | false |

### Environment Configuration

The CLI automatically loads environment variables from a `.env` file in the current directory (if it exists). This is useful for setting default values for flags without typing them on the command line.

**Example `.env` file:**

```bash
# Log configuration
SCRIPTLING_LOG_LEVEL=debug
SCRIPTLING_LOG_FORMAT=console

# Library directory
SCRIPTLING_LIBDIR=./mylibs

# Server configuration
SCRIPTLING_SERVER=:8000
SCRIPTLING_MCP_TOOLS=./tools
SCRIPTLING_BEARER_TOKEN=your-secret-token
SCRIPTLING_SCRIPT_MODE=full

# Filesystem restrictions
SCRIPTLING_ALLOWED_PATHS=/tmp/data,./uploads
```

### Script Execution Modes

Scriptling supports three levels of filesystem access control:

| Mode | Flag | Filesystem Access | Path Restrictions |
|------|------|-------------------|-------------------|
| **Full** | `--script-mode full` (default) | All libraries | None |
| **Restricted** | `--allowed-paths /path1,/path2` | All libraries | Only specified paths |
| **Safe** | `--script-mode safe` | No filesystem libraries | N/A |

#### Full Mode (default)

All libraries available, no restrictions:

```bash
scriptling script.py
```

#### Restricted Mode

All libraries available, but filesystem operations restricted to specified paths:

```bash
# Restrict to specific directories
scriptling --allowed-paths "/tmp/data,./uploads" script.py

# With relative paths
scriptling --allowed-paths "./data,../shared" script.py

# Via environment variable
SCRIPTLING_ALLOWED_PATHS="/var/www,./public" scriptling script.py
```

When a script tries to access a path outside the allowed directories:

```python
import os
# This will raise an error if /etc/passwd is not in allowed paths
try:
    content = os.read_file("/etc/passwd")
except Exception as e:
    print(f"Access denied: {e}")
    # Output: Access denied: access denied: path '/etc/passwd' is outside allowed directories
```

#### Safe Mode

Only safe libraries available (no filesystem, subprocess, or network access libraries):

```bash
scriptling --script-mode safe script.py
```

**Safe mode libraries include:**

- Standard libraries: `json`, `math`, `random`, `re`, `time`, `base64`, `hashlib`, `urllib`
- `datetime` - Date and time operations
- `yaml` - YAML parsing
- `html.parser` - HTML parsing
- `requests` - HTTP client
- `os` - Environment variables access only
- `secrets` - Cryptographic random number generation
- `scriptling.kv` - Key-value store
- AI, agent, and MCP libraries

**Full mode additionally includes:**

- `subprocess` - Process execution
- `pathlib`, `glob` - File system access
- `threads` - Multi-threading
- `wait_for` - Process monitoring

### Accessing Environment Variables in Scripts

You can access environment variables from within Scriptling scripts using the `os` library:

```python
import os

# Get a specific environment variable
api_key = os.getenv("API_KEY", "default-key")
print(f"API Key: {api_key}")

# Get all environment variables
env = os.environ()
print(f"Home: {env['HOME']}")
print(f"Path: {env['PATH']}")
```

### HTTP Server Mode

Start an HTTP server with optional MCP tools:

```bash
# Start HTTP server on port 8000
scriptling --server :8000 setup.py

# With TLS (self-signed certificate)
scriptling --server :8443 --tls-generate setup.py

# With MCP tools
scriptling --server :8000 --mcp-tools ./tools setup.py

# With authentication
scriptling --server :8000 --bearer-token my-secret-token setup.py

# With filesystem restrictions
scriptling --server :8000 --allowed-paths "/var/www,./uploads" setup.py

# Safe mode (no filesystem access)
scriptling --server :8000 --script-mode safe setup.py
```

#### MCP Tools

Tools consist of two files: a `.toml` metadata file and a `.py` script file.

##### Metadata File (`.toml`)

Defines the tool's description, parameters, and registration mode:

**hello.toml:**

```toml
description = "Greet a person by name"
keywords = ["hello", "greet", "welcome"]

# Optional: Registration mode
# discoverable = false  (default) - Native mode: tool appears in tools/list, directly callable
# discoverable = true             - Discovery mode: hidden from tools/list, found via tool_search

[[parameters]]
name = "name"
type = "string"
description = "Name of the person to greet"
required = true

[[parameters]]
name = "times"
type = "int"
description = "Number of times to repeat the greeting"
```

**Parameter Types:**

| Type | Aliases | Description |
|------|---------|-------------|
| `string` | | Text values |
| `int` | `integer` | Integer numbers |
| `float` | `number` | Floating point numbers |
| `bool` | `boolean` | True/false values |
| `array:string` | | Array of strings |
| `array:number` | `array:int`, `array:integer`, `array:float` | Array of numbers |
| `array:bool` | `array:boolean` | Array of booleans |

**Parameter Fields:**

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Parameter name |
| `type` | Yes | Data type (see table above) |
| `description` | Yes | Description shown to the LLM |
| `required` | No | Whether the parameter must be provided (default: `false`) |

**Registration Modes:**

- **Native mode** (default): Tool appears in `tools/list` and can be called directly
- **Discovery mode** (`discoverable = true`): Tool is hidden from `tools/list`, searchable via `tool_search`, and callable via `execute_tool`

##### Script File (`.py`)

Implements the tool logic using the `scriptling.mcp.tool` library:

**hello.py:**

```python
import scriptling.mcp.tool as tool

# Get parameters with defaults
name = tool.get_string("name", "World")
times = tool.get_int("times", 1)

# Implement tool logic
greetings = []
for i in range(times):
    greetings.append(f"Hello, {name}!")

result = "\n".join(greetings)

# Return result
tool.return_string(result)
```

**Available Tool Functions:**

```python
# Get parameters
tool.get_string(name, default="")     # Get string parameter
tool.get_int(name, default=0)         # Get integer parameter
tool.get_float(name, default=0.0)     # Get float parameter
tool.get_bool(name, default=False)    # Get boolean parameter
tool.get_list(name, default=[])       # Get list parameter

# Return results
tool.return_string(text)              # Return text result
tool.return_object(obj)               # Return object as JSON
tool.return_toon(obj)                 # Return object as TOON format
tool.return_error(message)            # Return error message
```

##### Complete Example

A tool that calculates the sum of two numbers:

**add.toml:**

```toml
description = "Calculate the sum of two numbers"
keywords = ["math", "add", "sum", "calculate"]
discoverable = true  # Hidden from tools/list, searchable

[[parameters]]
name = "a"
type = "int"
description = "First number"
required = true

[[parameters]]
name = "b"
type = "int"
description = "Second number"
required = true
```

**add.py:**

```python
import scriptling.mcp.tool as tool

a = tool.get_int("a")
b = tool.get_int("b")

result = a + b
tool.return_string(f"{a} + {b} = {result}")
```

#### Testing Tools

```bash
# Start server with MCP tools
scriptling --server :8000 --mcp-tools ./tools setup.py &

# List available tools
curl -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'

# Call a native tool directly
curl -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"hello","arguments":{"name":"Alice","times":2}}}'

# Search for discoverable tools
curl -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"tool_search","arguments":{"query":"math"}}}'

# Execute a discoverable tool
curl -X POST http://127.0.0.1:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"execute_tool","arguments":{"name":"add","arguments":{"a":5,"b":3}}}}'
```

### Help

```bash
scriptling --help
```

## Building

The CLI tool uses [Task](https://taskfile.dev/) for building. Install Task first:

```bash
# macOS
brew install go-task/tap/go-task

# Or download from https://taskfile.dev/
```

### Build for current platform

```bash
task build
```

### Build for all platforms

```bash
task build-all
```

### Install locally

```bash
task install
```

## Features

- **File execution**: Run Scriptling scripts from files
- **Stdin execution**: Pipe scripts to stdin
- **Interactive mode**: REPL-like interactive execution
- **Lint mode**: Check scripts for syntax errors without execution
- **HTTP Server**: Start HTTP server with custom routes via `--server`
- **MCP Server**: Serve tools via Model Context Protocol with `--mcp-tools`
- **Sandboxed execution**: Safe mode restricts access to file system, network, and subprocess
- **Custom libraries**: Load libraries from custom directories with `--libdir`
- **Environment configuration**: Auto-load settings from `.env` file
- **Configurable logging**: Set log level with `--log-level` (debug, info, warn, error)
- **Cross-platform**: Built for Linux, macOS, and Windows on AMD64 and ARM64
- **Minimal size**: Optimized with stripped binaries (~7MB)

## Libraries

The CLI includes all standard libraries plus external libraries:

- `datetime`, `json`, `math`, `random`, `re`, `time`, `base64`, `hashlib`, `urllib`
- `requests` - HTTP client library
- `subprocess` - Process execution library
