# Scriptling MCP Server

An MCP (Model Context Protocol) server that executes Scriptling/Python code with tool discovery.

## Tools

### Visible Tools

- **execute_script** - Execute custom Scriptling/Python code

### Discovery Tools

- **tool_search** - Search for pre-built tools by keywords
- **execute_tool** - Execute a discovered tool by name

### Pre-built Tools (via discovery)

- **generate_calendar** - Generate ASCII calendars for any month/year
- **generate_password** - Generate secure random passwords
- **http_post_json** - Send JSON POST requests

## Usage

```bash
cd examples/mcp
go build
./scriptling-mcp-server
```

Server runs on port 8080 at `/mcp`.

## Example

```bash
# Search for tools
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"tool_search","arguments":{"query":"calendar"}}}'

# Execute a tool
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"execute_tool","arguments":{"name":"generate_calendar","arguments":{"year":2025,"month":12}}}}'
```