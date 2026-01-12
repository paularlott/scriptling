# Scriptling Examples

This directory contains examples and tools for working with Scriptling.

## Structure

- **scripts/** - Script examples and test files
  - `main.go` - Go integration example for running scripts
  - `*.py` - Various Scriptling example scripts (basics, features, libraries, etc.)
  - `run_all_examples.sh` - Script to run all examples

- **mcp/** - MCP (Model Context Protocol) server for LLM testing
  - `main.go` - MCP server implementation
  - `README.md` - MCP server documentation

- **mcp-client/** - MCP client examples
  - `with-openai/` - Using MCP tools through an OpenAI client
  - `direct/` - Direct MCP server connection
  - `README.md` - MCP client examples documentation

- **openai/** - AI library examples with OpenAI-compatible APIs
  - `shared/` - Using shared client configured in Go
  - `instance/` - Creating client from script
  - `README.md` - OpenAI examples documentation

- **extending/** - Example of extending Scriptling with custom Go functions
  - `main.go` - Go integration example
  - `README.md` - Extension documentation

- **logging/** - Example of using the logging library
  - `main.go` - Go integration with logging
  - `example.py` - Scriptling logging example

- **multi-environment/** - Example of using multiple isolated Scriptling environments
  - `main.go` - Go integration example
  - `README.md` - Multi-environment documentation

## Running Script Examples

```bash
cd scripts
go run main.go example_basics.py
go run main.go test_all_features.py
./run_all_examples.sh
```

## AI/MCP Examples

### OpenAI Examples

Examples demonstrating the AI library with OpenAI-compatible APIs (including LM Studio).

```bash
# Shared client pattern (client configured in Go)
cd openai/shared
go run main.go

# Instance pattern (client created from script)
cd openai/instance
go run main.go
```

See [openai/README.md](openai/README.md) for details and prerequisites.

### MCP Client Examples

Examples demonstrating MCP (Model Context Protocol) client usage.

```bash
# Using MCP tools through an OpenAI client
cd mcp-client/with-openai
go run main.go

# Direct MCP server connection
cd mcp-client/direct
go run main.go
```

**Prerequisites**: Start the MCP server first:
```bash
cd mcp
go run main.go
```

## Example Scripts

The `scripts/` directory contains numerous examples covering:

- **Basics**: `example_basics.py`, `example_control_flow.py`, `example_loops.py`
- **Functions**: `example_functions.py`, `example_lambda.py`, `example_variadic_args.py`
- **Data Types**: `example_tuples.py`, `example_collections.py`, `example_list_comprehensions.py`
- **Libraries**: `example_lib_*.py` - Examples for various libraries (json, regex, math, etc.)
- **HTTP**: `example_lib_http.py`, `rest_api.py`
- **Async**: `example_async.py`
- **And many more...**

## MCP Server for LLM Testing

The MCP server allows LLMs to execute Scriptling code and learn about language differences:

```bash
cd mcp
go mod tidy
go run main.go
```

See `mcp/README.md` for detailed usage instructions.

## Other Examples

See individual example directories for more details:
- [extending/README.md](extending/README.md) - Extending Scriptling with Go
- [logging/](logging/) - Logging library example
- [multi-environment/README.md](multi-environment/README.md) - Multi-environment usage
- [openai/README.md](openai/README.md) - AI library with OpenAI-compatible APIs
