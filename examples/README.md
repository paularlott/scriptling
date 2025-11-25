# Scriptling Examples

This directory contains examples and tools for working with Scriptling.

## Structure

- **scripts/** - Script examples and test files
  - `main.go` - Go integration example for running scripts
  - `*.py` - Various Scriptling example scripts
  - `run_all_tests.sh` - Script to run all test examples

- **mcp/** - MCP (Model Context Protocol) server for LLM testing
  - `main.go` - MCP server implementation
  - `README.md` - MCP server documentation

## Running Script Examples

```bash
cd scripts
go run main.go basic.py
go run main.go test_error_comprehensive.py
./run_all_tests.sh
```

## MCP Server for LLM Testing

The MCP server allows LLMs to execute Scriptling code and learn about language differences:

```bash
cd mcp
go mod tidy
go run main.go
```

See `mcp/README.md` for detailed usage instructions.