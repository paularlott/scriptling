#!/usr/bin/env scriptling
"""Example demonstrating TCP echo server and client using scriptling.net.unicast"""

import scriptling.net.unicast as uc

print("=== TCP Echo Server & Client ===\n")

# Start a TCP server on a random port
server = uc.listen("127.0.0.1", 0, protocol="tcp")
port = int(server.addr.split(":")[1])
print(f"Server listening on {server.addr}")

# Connect a client
conn = uc.connect("127.0.0.1", port, protocol="tcp", timeout=5)
print(f"Client connected to {conn.remote_addr}")

# Accept the connection on the server side
sc = server.accept(timeout=5)
print(f"Server accepted connection from {sc.remote_addr}")

# Client sends a message
conn.send("hello from client")
msg = sc.receive(timeout=5)
print(f"Server received: {msg['data']}")

# Server echoes back
sc.send("echo: " + msg["data"])
reply = conn.receive(timeout=5)
print(f"Client received: {reply['data']}")

# Send a dict (auto-encoded as JSON)
conn.send({"action": "ping", "count": 42})
raw = sc.receive(timeout=5)
print(f"Server received JSON: {raw['data']}")

# Check connection state
print(f"\nClient connected: {conn.connected()}")
conn.close()
sc.close()
server.close()
print(f"Client connected after close: {conn.connected()}")

print("\n=== TCP Example Complete ===")
