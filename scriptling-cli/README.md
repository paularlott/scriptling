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

### Custom library directory
```bash
scriptling --libdir ./mylibs script.py
```

### MCP Server

Start an MCP (Model Context Protocol) server to serve tools:

```bash
# Start server (default: 127.0.0.1:8000, tools: ./tools)
scriptling mcp serve

# Custom configuration
scriptling mcp serve --address 0.0.0.0:9000 --tools ./my-tools

# With authentication
scriptling mcp serve --bearer-token my-secret-token

# With custom library directory
scriptling mcp serve --libdir ./mylibs

# Validate tools without starting server
scriptling mcp serve --validate --tools ./tools

# Set log level
scriptling --log-level debug mcp serve
```

#### Authoring Tools

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
type = "string"           # Supported: string, int, integer, float, number, bool, boolean
description = "Name of the person to greet"
required = true

[[parameters]]
name = "times"
type = "int"
description = "Number of times to repeat the greeting"
required = false
```

**Parameter Types:**
- `string` - Text values
- `int`, `integer` - Integer numbers
- `float`, `number` - Floating point numbers
- `bool`, `boolean` - True/false values

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
# Validate tool metadata and scripts
scriptling mcp serve --validate --tools ./tools

# Start server and test with curl
scriptling mcp serve --tools ./tools &

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
- **MCP Server**: Serve tools via Model Context Protocol
- **Custom libraries**: Load libraries from custom directories with `--libdir`
- **Configurable logging**: Set log level with `--log-level` (debug, info, warn, error)
- **Cross-platform**: Built for Linux, macOS, and Windows on AMD64 and ARM64
- **Minimal size**: Optimized with stripped binaries (~7MB)

## Libraries

The CLI includes all standard libraries plus external libraries:
- `datetime`, `json`, `math`, `random`, `re`, `time`, `base64`, `hashlib`, `urllib`
- `requests` - HTTP client library
- `subprocess` - Process execution library