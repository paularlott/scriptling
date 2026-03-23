# WebSocket Examples

This directory contains examples demonstrating WebSocket support in Scriptling.

## Files

- `echo_server.py` - Server setup that registers a WebSocket echo endpoint
- `handlers.py` - Handler functions for the server
- `client.py` - Client script that connects to the echo server

## Running the Server

```bash
scriptling --server :8080 -L examples/websocket examples/websocket/echo_server.py
```

The server will start on http://localhost:8080.

## Running the Client

In another terminal, connect to the server:

```bash
scriptling -L examples/websocket examples/websocket/client.py
```

## What the Example Does

1. The server registers a WebSocket endpoint at `/echo`
2. The client connects and receives a welcome message
3. The client sends several messages which the server echoes back
4. The client sends a JSON object (dict) which is automatically encoded
5. The client closes the connection
