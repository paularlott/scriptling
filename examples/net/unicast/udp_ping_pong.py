#!/usr/bin/env scriptling
"""Example demonstrating UDP ping-pong using scriptling.net.unicast"""

import scriptling.net.unicast as uc

print("=== UDP Ping-Pong ===\n")

# Start a UDP server on a random port
server = uc.listen("127.0.0.1", 0, protocol="udp")
port = int(server.addr.split(":")[1])
print(f"UDP server listening on {server.addr}")

# Connect a UDP client
conn = uc.connect("127.0.0.1", port, protocol="udp")
print(f"UDP client connected to {conn.remote_addr}")

# Client sends ping
conn.send("ping")
print("Client sent: ping")

# Server receives and replies
msg = server.receive(timeout=5)
print(f"Server received: {msg['data']} from {msg['source']}")
server.send_to(msg["source"], "pong")

# Client receives pong
reply = conn.receive(timeout=5)
print(f"Client received: {reply['data']}")

# Send multiple messages quickly
for i in range(3):
    conn.send(f"message-{i}")

print("\nServer receiving batch:")
for i in range(3):
    msg = server.receive(timeout=2)
    if msg:
        print(f"  Got: {msg['data']}")

conn.close()
server.close()

print("\n=== UDP Example Complete ===")
