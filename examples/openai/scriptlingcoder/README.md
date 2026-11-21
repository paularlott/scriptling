# Scriptlingcoder - AI Coding Assistant

A small coding assistant for the terminal, written in Scriptling. It gives a model tools to read, search and edit files and to run shell commands, and runs them in the agent loop from `scriptling.ai.agent`. Inspired by [nanocode](https://github.com/1rgs/nanocode).

**⚠️ WARNING**: This example executes AI-generated code and shell commands without asking first. It may modify or delete files. Use at your own risk!

## Features

- **File tools**: read (with line numbers), write and edit files
- **Search**: find files by glob pattern and search their contents by regular expression
- **Shell**: run commands, with a timeout and the exit code reported
- **MCP servers**: tools and skills from any servers listed in `.mcp.json`
- **Project instructions**: an `AGENTS.md` in the working directory is added to the system prompt
- **Long sessions**: older history is summarised as the conversation nears the model's context window

## Prerequisites

1. **An OpenAI-compatible API** with a model that supports tool calling, for example [LM Studio](https://lmstudio.ai/) on `127.0.0.1:1234` or [Ollama](https://ollama.com/) on `127.0.0.1:11434`.

2. **Scriptling CLI**
   ```bash
   cd ../../..
   task build
   ```

## Usage

Run it from the directory of the project you want to work on:

```bash
OPENAI_BASE_URL=http://127.0.0.1:11434/v1 OPENAI_MODEL=your-model \
  /path/to/scriptling /path/to/scriptlingcoder.py
```

### Environment Variables

| Variable | Default | Purpose |
|---|---|---|
| `OPENAI_BASE_URL` | `http://127.0.0.1:1234/v1` | API endpoint |
| `OPENAI_MODEL` | `mistralai/ministral-3-3b` | Model name |
| `OPENAI_API_KEY` | empty | API key, if the endpoint needs one |
| `OPENAI_CONTEXT_TOKENS` | `32000` | The model's context window in tokens |

Set `OPENAI_CONTEXT_TOKENS` to what the model really has. The agent summarises older history at 80% of this figure, so a value that is too low throws away context the model could have kept, and one that is too high lets the server silently drop the start of the conversation.

### Commands

- Type your request and press Enter
- `Esc` - Interrupt the current request (work already done is kept in the conversation)
- `/clear` - Clear conversation history and screen
- `/model <name>` - Switch model
- `/history` - Show conversation history
- `/exit` - Quit

## Available Tools

- **read** `path, offset?, limit?` - Read a file as numbered lines, 200 at a time; `offset` is the first line to return
- **write** `path, content` - Create or replace a file
- **edit** `path, old_string, new_string, replace_all?` - Replace text in a file (`old_string` must be unique unless `replace_all` is true)
- **glob** `pattern, path?` - Find files by pattern, newest first
- **grep** `pattern, path?, glob?` - Search file contents for a regular expression (up to 100 matches)
- **bash** `command, timeout?` - Run a shell command (120 second timeout by default)

Search skips `.git`, `node_modules`, `vendor`, `dist`, `build`, `bin`, `__pycache__` and `.venv`, and grep also skips binary files and files over 1 MB.

## MCP Servers

To give the assistant more tools, put a `.mcp.json` in the directory you run it from:

```json
{
  "mcpServers": {
    "shop": {
      "command": "scriptling",
      "args": ["--mcp-tools", "./tools"],
      "env": {"SHOP_REGION": "au"}
    },
    "docs": {
      "url": "https://example.com/mcp",
      "bearer_token": "your-token"
    }
  }
}
```

An entry with `command` is started as a subprocess and spoken to over stdio; an entry with `url` is reached over HTTP. Each server's tools appear to the model as `<name>__<tool>`, and its skills are listed in the system prompt. The start-up banner shows which servers connected and how many tools each provides; a server that fails to start is reported there and skipped.

## How It Works

1. `build_tools()` registers the tools in an `ai.ToolRegistry`, each with a name, a description, its parameters and a handler function.
2. `interact.Agent` is given the client, the registry, any MCP servers and the system prompt, and `bot.interact()` runs the terminal UI.
3. For each request the agent sends the conversation to the model, runs the tool calls it asks for, sends the results back, and repeats until the model answers without calling a tool (or 25 rounds have passed).
4. A call with a missing required parameter is answered with a message naming the parameters the tool accepts, so the model can correct it; a very long tool result is shortened to its start and end.

## Customization

### Add a Tool

Write a handler that takes the arguments as a dict and returns a string, and register it in `build_tools()`:

```python
def word_count(args):
    return str(len(os.read_file(args["path"]).split()))

tools.add("word_count", "Count the words in a file.", {"path": "string", "encoding": "string?"}, word_count)
```

A `?` after the type marks a parameter as optional. Use the parameter names models expect (`path`, `command`, `pattern`): a model will often send the conventional name whatever the schema says.

### Change the System Prompt

Edit `SYSTEM_PROMPT` at the top of the script, or add an `AGENTS.md` to your project for instructions that belong to the project rather than the tool.

## Security Considerations

This is an example, not a hardened tool. The model can read and write any file the process can, and run any command, with no confirmation step. Before using anything like it on work that matters:

- Restrict file operations to the project directory
- Ask before running shell commands and writing files, or remove the bash tool
- Run it in a container or other sandbox
- Only list MCP servers you trust in `.mcp.json`: a `command` entry is run as a program on your machine

## Troubleshooting

**Connection refused**: check the API server is running and `OPENAI_BASE_URL` points at it

**Model not found**: check `OPENAI_MODEL` matches a model the server has loaded

**The model never calls tools, or calls them badly**: use a model trained for tool calling; very small models often are not reliable enough

**The model seems to forget earlier work**: raise `OPENAI_CONTEXT_TOKENS` if the model's context window is larger than the default

## Related Examples

- `../agent/` - The agent loop without the terminal UI
- `../streaming/` - Streaming responses
- `../../agent-mcp/` - An agent using an MCP server's tools and skills

## License

MIT - See main repository LICENSE
