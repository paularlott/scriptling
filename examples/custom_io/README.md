# Custom I/O Streams Example

This example demonstrates how to use custom input and output streams with Scriptling for remote execution scenarios.

## Overview

Scriptling supports custom I/O streams, allowing you to:
- Redirect `print()` output to any `io.Writer` (files, websockets, loggers, etc.)
- Redirect `console.input()` from any `io.Reader` (strings, files, websockets, etc.)
- Run multiple scripts in parallel with separate I/O streams
- Stream script execution over network connections

## Running the Example

```bash
go run main.go
```

## Examples Included

### Example 1: Basic Custom I/O
Demonstrates simple input/output redirection using strings and buffers.

### Example 2: Simulated WebSocket
Shows how to simulate remote script execution with bidirectional I/O, similar to a websocket connection.

### Example 3: Multiple Parallel Sessions
Demonstrates running multiple script instances in parallel, each with their own I/O streams.

## Use Cases

- **Remote Script Execution**: Execute scripts on a server and stream I/O to/from clients
- **Testing**: Capture output and provide input programmatically
- **Logging**: Redirect script output to custom loggers
- **Web Applications**: Stream script execution to web clients via websockets
- **Interactive Tools**: Build CLI tools with custom I/O handling

## API

```go
// Set custom output writer
p.SetOutputWriter(writer io.Writer)

// Set custom input reader
p.SetInputReader(reader io.Reader)

// Enable simple output capture (uses strings.Builder)
p.EnableOutputCapture()
output := p.GetOutput()
```

## Notes

- Each Scriptling instance maintains its own I/O streams
- I/O streams are inherited by nested environments (functions, classes)
- Defaults to `os.Stdin` and `os.Stdout` if not set
- Safe for concurrent use with separate instances
