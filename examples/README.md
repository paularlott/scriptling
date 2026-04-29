# Scriptling Examples

This directory contains examples and tools for working with Scriptling.

## Structure

- **scripts/** - Script examples and test files
  - `main.go` - Go integration example for running scripts
  - `*.py` - Various Scriptling example scripts (basics, features, libraries, etc.)
  - `run_all_examples.sh` - Script to run all examples

- **background/** - Runtime library background tasks and concurrency example
  - `example.py` - Demonstrates background tasks, runtime.run(), sync primitives, KV store
  - `README.md` - Background tasks documentation

- **mcp-client/** - MCP client examples
  - `with-openai/` - Using MCP tools through an OpenAI client
  - `direct/` - Direct MCP server connection
  - `README.md` - MCP client examples documentation

- **openai/** - AI library examples with OpenAI-compatible APIs
  - `shared/` - Using shared client configured in Go
  - `instance/` - Creating client from script
  - `streaming/` - Streaming chat completions
  - `scriptlingcoder/` - AI coding assistant with custom tools (⚠️ executes AI code)
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

- **call_method/** - Example of calling methods on Scriptling objects from Go
  - `main.go` - Go integration example
  - `README.md` - Method calling documentation

- **custom_io/** - Example of custom input/output handling
  - `main.go` - Go integration example
  - `README.md` - Custom I/O documentation

- **http-server/** - Example of using the HTTP server library
  - `main.go` - Go integration example
  - `README.md` - HTTP server documentation

- **net/** - Networking library examples (`scriptling.net.*`)
  - **net/multicast/** - UDP multicast group messaging
    - `multicast_demo.py` - Join a multicast group, send and receive messages
    - `README.md` - Multicast library documentation
  - **net/unicast/** - UDP and TCP point-to-point messaging
    - `tcp_echo.py` - TCP echo server and client
    - `udp_ping_pong.py` - UDP ping-pong example
    - `README.md` - Unicast library documentation
  - **net/gossip/** - Gossip protocol cluster membership and messaging
    - `gossip_demo.py` - Single-node metadata and introspection demo
    - `gossip_cluster.py` - Two-node cluster with message passing
    - `gossip_nodegroups.py` - Metadata-criteria-based node groups
    - `gossip_leader.py` - Leader election and failover
    - `gossip_request_reply.py` - Request/reply messaging pattern
    - `gossip_advanced.py` - Advanced config and event handlers
    - `README.md` - Gossip library documentation

- **mcp-tools/** - Example MCP tools for use with the CLI MCP server
  - `*.toml` - Tool metadata files
  - `*.py` - Tool implementation scripts
  - `README.md` - MCP tools documentation

- **telegram-bot/** - Example Telegram bot using Scriptling
  - `main.go` - Go integration example
  - `README.md` - Telegram bot documentation

- **fs/** - Binary file I/O with the `fs` library
  - `read_gguf.py` - Parse GGUF (LLM model) file headers
  - `read_wav.py` - Parse WAV audio file headers
  - `read_png.py` - Parse PNG image file headers
  - `README.md` - fs library documentation

- **math/** - Linear algebra and advanced math functions
  - `neural_network.py` - Neural network forward pass using matmul, tanh, softmax
  - `matrix_ops.py` - Matrix multiplication, transpose, addition, dot product
  - `README.md` - math library documentation

- **random/** - Random number generation and distributions
  - `distributions.py` - Weighted sampling, Monte Carlo, distribution sampling, card dealing
  - `README.md` - random library documentation

## Running Script Examples

```bash
cd scripts
go run main.go example_basics.py
go run main.go test_all_features.py
./run_all_examples.sh
```

## Runtime Library Example

Demonstrates background tasks, concurrent execution, synchronization primitives, and KV store:

```bash
# Build CLI first (from repo root)
task build

# Run the example
./bin/scriptling examples/background/example.py
```

See [background/README.md](background/README.md) for details.

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

# Streaming responses
cd openai/streaming
go run main.go

# AI coding assistant with custom tools (⚠️ WARNING: executes AI-generated code)
cd openai/scriptlingcoder
../../../bin/scriptling scriptlingcoder.py
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

**Prerequisites**: Start an MCP server first (e.g., using the CLI with `--server :8000 --mcp-tools ./tools`).

## Example Scripts

The `scripts/` directory contains numerous examples covering:

- **Basics**: `example_basics.py`, `example_control_flow.py`, `example_loops.py`
- **Functions**: `example_functions.py`, `example_lambda.py`, `example_variadic_args.py`
- **Data Types**: `example_tuples.py`, `example_collections.py`, `example_list_comprehensions.py`
- **Libraries**: `example_lib_*.py` - Examples for various libraries (json, regex, math, etc.)
- **HTTP**: `example_lib_http.py`, `rest_api.py`
- **Async**: `example_async.py`
- **And many more...**

## Other Examples

See individual example directories for more details:
- [background/README.md](background/README.md) - Background tasks and concurrency
- [call_method/README.md](call_method/README.md) - Calling methods on Scriptling objects
- [custom_io/README.md](custom_io/README.md) - Custom input/output handling
- [extending/README.md](extending/README.md) - Extending Scriptling with Go
- [http-server/README.md](http-server/README.md) - HTTP server library
- [logging/](logging/) - Logging library example
- [mcp-client/README.md](mcp-client/README.md) - MCP client examples
- [mcp-tools/README.md](mcp-tools/README.md) - MCP tools examples
- [multi-environment/README.md](multi-environment/README.md) - Multi-environment usage
- [openai/README.md](openai/README.md) - AI library with OpenAI-compatible APIs
- [telegram-bot/README.md](telegram-bot/README.md) - Telegram bot example
- [net/unicast/README.md](net/unicast/README.md) - UDP and TCP point-to-point messaging
- [net/multicast/README.md](net/multicast/README.md) - UDP multicast group messaging
- [net/gossip/README.md](net/gossip/README.md) - Gossip protocol cluster membership and messaging
- [fs/README.md](fs/README.md) - Binary I/O: pack, unpack, read/write bytes
- [math/README.md](math/README.md) - Linear algebra: matmul, dot, transpose, softmax, special functions
- [random/README.md](random/README.md) - Distributions: gauss, beta, gamma, pareto, weibull, choices
