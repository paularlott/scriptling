# Scriptling MCP Server

An MCP (Model Context Protocol) server that allows LLMs to test and interact with the Scriptling language.

## Tools

### execute_scriptling
Execute Scriptling code and get the output.

**Parameters:**
- `code` (string): The Scriptling code to execute

**Returns:**
- `output` (string): Captured print output
- `result` (string, optional): Final expression result
- `error` (string, optional): Error message if execution failed

### scriptling_info
Get information about Scriptling language differences from Python and available libraries.

**Parameters:**
- `info_type` (string): Type of information - "differences", "libraries", or "all"

**Returns:**
- `differences` (object, optional): Syntax and type differences from Python
- `libraries` (object, optional): Available built-in and optional libraries

## Usage

```bash
# Build and run the server
cd examples/mcp
go mod tidy
go run main.go
```

The server will start on port 8080 and accept MCP requests at `/mcp` endpoint.

## Example Interactions

### Execute Code
```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "method": "tools/call",
    "params": {
      "name": "execute_scriptling",
      "arguments": {
        "code": "print(\"Hello from Scriptling!\")\nx = 5 + 3\nprint(x)"
      }
    }
  }'
```

### Get Language Info
```bash
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{
    "method": "tools/call",
    "params": {
      "name": "scriptling_info",
      "arguments": {
        "info_type": "all"
      }
    }
  }'
```