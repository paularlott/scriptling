# Scriptlingcoder - AI Coding Assistant

An AI-powered coding assistant that can read, write, and modify files using custom tools. Inspired by [nanocode](https://github.com/1rgs/nanocode).

**⚠️ WARNING**: This is an example that executes AI-generated code and shell commands. It may modify or delete files. Use at your own risk!

## Features

- **File Operations**: Read, write, and edit files
- **Search**: Find files with glob patterns and search content with regex
- **Shell Commands**: Execute bash commands
- **Interactive**: Chat-based interface with conversation history
- **Custom Tools**: Demonstrates how to use custom tools with the AI library

## Prerequisites

1. **LM Studio** (or any OpenAI-compatible API)

   - Download from [lmstudio.ai](https://lmstudio.ai/)
   - Start the server on `127.0.0.1:1234`
   - Load a model (e.g., `qwen3-coder-30b-a3b-instruct-mlx`)

2. **Scriptling CLI**
   ```bash
   cd ../../..
   task build
   ```

## Usage

### Basic Usage

```bash
# Run with default settings (LM Studio on localhost:1234)
../../../bin/scriptling scriptlingcoder.py
```

### Environment Variables

Configure the AI connection:

```bash
# Use a different base URL
export OPENAI_BASE_URL="http://localhost:8080/v1"

# Use a different model
export OPENAI_MODEL="gpt-4"

# Set API key (if required)
export OPENAI_API_KEY="your-api-key"

# Run
../../../bin/scriptling scriptlingcoder.py
```

### Commands

- Type your request and press Enter
- `/c` - Clear conversation history
- `/q` or `exit` - Quit

## Example Session

```
scriptlingcoder | mistralai/ministral-3-3b | /path/to/project
⚠ WARNING: This tool executes AI-generated code. Use at your own risk!

────────────────────────────────────────────────────────────────────────────────
❯ List all Python files in the current directory
────────────────────────────────────────────────────────────────────────────────

⏺ Glob(*.py)
  ⎿  main.py ... +5 lines

⏺ Here are the Python files in the current directory:
- main.py
- test.py
- utils.py
...

────────────────────────────────────────────────────────────────────────────────
❯ Read the first 10 lines of main.py
────────────────────────────────────────────────────────────────────────────────

⏺ Read(main.py)
  ⎿     1| #!/usr/bin/env python3 ... +9 lines

⏺ Here are the first 10 lines of main.py:
...
```

## Available Tools

The AI has access to these tools:

- **read** - Read file with line numbers (supports offset and limit)
- **write** - Write content to a file
- **edit** - Replace text in a file (old string must be unique unless all=true)
- **glob** - Find files by pattern, sorted by modification time
- **grep** - Search files for regex pattern (max 50 results)
- **bash** - Run shell command

## How It Works

1. **Custom Tools**: The script defines tools and passes them via `tools` parameter to `completion()`
2. **AI Calls**: When you send a message, the AI can choose to call tools
3. **Tool Execution**: The script executes the tools locally and returns results
4. **Iteration**: The AI receives tool results and can make additional calls
5. **Response**: The AI provides a final response based on tool results

## Security Considerations

This example is for demonstration purposes. In production:

- ✅ Validate all file paths (prevent directory traversal)
- ✅ Restrict file operations to specific directories
- ✅ Sanitize shell commands or disable bash tool
- ✅ Add rate limiting and timeouts
- ✅ Log all operations for audit
- ✅ Run in a sandboxed environment

## Customization

### Add New Tools

```python
def my_tool(args):
    # Your implementation
    return "result"

TOOLS["mytool"] = [
    "Tool description",
    {"param1": "string", "param2": "number?"},  # ? = optional
    my_tool
]
```

### Modify System Prompt

Edit the `system_prompt` variable to change the AI's behavior:

```python
system_prompt = "You are a helpful coding assistant specialized in Python. cwd: " + os.getcwd()
```

## Troubleshooting

**Connection refused**: Make sure LM Studio (or your API) is running

**Model not found**: Verify the model is loaded in LM Studio

**Tool errors**: Check file permissions and paths

**Empty responses**: Try a different model or adjust the prompt

## Related Examples

- `../instance/` - Basic OpenAI client usage
- `../streaming/` - Streaming responses
- `../../mcp-client/` - Using MCP servers with AI

## License

MIT - See main repository LICENSE
