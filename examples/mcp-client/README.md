# MCP Client Examples

This directory contains examples demonstrating how to use the MCP (Model Context Protocol) client library with Scriptling.

## Prerequisites

**Start the Scriptling MCP Server:**
```bash
cd ../../mcp
go run main.go
```
The server runs on `http://127.0.0.1:8080/mcp`

**For OpenAI examples, you also need LM Studio:**
- Install from [lmstudio.ai](https://lmstudio.ai/)
- Start the server on `127.0.0.1:1234`
- Load a model (e.g., `mistralai/ministral-3-3b`)

## Examples

### 1. shared/ - MCP Client Wrapped in Go

Creates the MCP client in Go, wraps it, and passes it to the script as a global variable.

```bash
cd shared
go run main.go
```

**How it works:**
- Go code creates an `mcp.Client` for the scriptling MCP server
- Client is wrapped via `mcp.WrapClient()` and set as a global variable via `p.SetObjectVar()`
- Script uses instance methods like `mcp_client.tools()` and `mcp_client.call_tool()` directly

**Use this pattern when:**
- You want to manage the MCP client configuration in Go
- Multiple scripts need to share the same MCP client
- You want to keep server URLs out of scripts
- You need to support multiple different MCP clients simultaneously

**What the script does:**
1. Lists available tools from the MCP client
2. Calls the `execute_code` tool to add two numbers
3. Searches for calendar-related tools
4. Executes a discovered tool to generate a calendar

### 2. direct/ - MCP Client Created in Script

Creates the MCP client directly in the script without any pre-configuration in Go.

```bash
cd direct
go run main.go
```

**How it works:**
- No client is configured in Go
- Script creates its own MCP client via `mcp.new_client()`
- Script uses instance methods to list and call tools directly
- Tools are called explicitly with specific arguments

**Use this pattern when:**
- You want scripts to be self-contained
- Each script needs different MCP client configurations
- You're writing scripts that can run standalone

**What the script does:**
1. Creates MCP client and lists available tools
2. Calls the `execute_code` tool to add two numbers
3. Searches for calendar-related tools
4. Executes a discovered tool to generate a calendar

### 3. with-openai/ - Both Clients in Go

Creates both OpenAI and MCP clients in Go, attaches the MCP server to the OpenAI client, and passes both to the script.

```bash
cd with-openai
go run main.go
```

**How it works:**
- Go code creates an `openai.Client` for LM Studio
- Go code creates an `mcp.Client` for the scriptling MCP server
- MCP server is attached to the OpenAI client via `client.AddRemoteServer()`
- Both clients are wrapped and passed as global variables
- Script can use MCP tools directly or via the AI

**Use this pattern when:**
- You want an AI model to use MCP tools
- You need automatic tool calling during chat completions
- You want the AI to decide which tools to use
- You want to manage all client configuration in Go

**What the script does:**
1. Lists available tools from the MCP client
2. Asks the AI to calculate 15 + 27 using the execute_code tool
3. The AI automatically calls the tool and returns the answer

### 4. with-openai-instance/ - Both Clients in Script

Creates both OpenAI and MCP clients in the script, with MCP servers attached to the AI client via the `remote_servers` parameter.

```bash
cd with-openai-instance
go run main.go
```

**How it works:**
- Script creates its own OpenAI client via `ai.new_client()` with `remote_servers` parameter
- Script creates its own MCP client via `mcp.new_client()` for direct tool access
- MCP servers are configured during AI client creation (no Go code needed)
- AI can automatically use tools from the configured MCP servers

**Use this pattern when:**
- You want scripts to be self-contained
- You're building standalone scripts that integrate AI with MCP tools
- You want the AI to use MCP tools in completions

**What the script does:**
1. Creates OpenAI client with MCP servers configured via `remote_servers`
2. Creates MCP client for direct tool listing
3. Lists available tools from the MCP client
4. Asks the AI to use MCP tools to calculate 15 + 27

## Available Tools

The Scriptling MCP server provides these tools:

- **execute_code** - Execute arbitrary Scriptling/Python code
- **tool_search** - Search for tools by keywords
- **execute_tool** - Execute a discovered tool by name
- **generate_calendar** - Generate an ASCII calendar for any month/year
- **generate_password** - Generate secure random passwords
- **http_post_json** - Send JSON POST requests

## Expected Output

### shared/ and direct/ Examples

```
Fetching available tools...
Found 3 tools:
  - execute_code: Execute Scriptling/Python code...
  - tool_search: Search for tools...
  - execute_tool: Execute a discovered tool...

Calling execute_code tool to add two numbers...

Result:
15 + 27 = 42

Searching for calendar-related tools...
Found 1 calendar tools:
  - generate_calendar: Generate a formatted ASCII calendar...

Executing the first calendar tool...
Calendar result:
     January 2025
...
```

### with-openai/ and with-openai-instance/ Examples

```
Fetching available tools from MCP server...
Found 3 tools:
  - scriptling/execute_code: Execute Scriptling/Python code...
  - scriptling/tool_search: Search for tools...
  - scriptling/execute_tool: Execute a discovered tool...
  - ...

Now asking the AI to use MCP tools...

Response:
The result of \(15 + 27\) is **42**.
```

**Note:** Tools are prefixed with "scriptling/" because the MCP client was created with that prefix. This ensures consistent naming whether tools are accessed directly through the MCP client or through the AI client.

## Troubleshooting

**Connection refused**: Make sure the MCP server is running on port 8080

**No tools found**: The MCP server may not be initialized - check the server logs

**AI doesn't call tools**: Some models may not support tool calling or need explicit instructions

**Model not found** (OpenAI examples): Make sure LM Studio is running and the model is loaded
