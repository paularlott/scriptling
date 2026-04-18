# WebSocket Client Example
# Run with: scriptling-cli run examples/websocket/client.py

import scriptling.net.websocket as ws

# Connect to the echo server
print("Connecting to WebSocket server...")
conn = ws.connect("ws://localhost:8080/echo", timeout=5)

# Receive welcome message
welcome = conn.receive(timeout=5)
print(f"Server says: {welcome}")

# Send some messages
messages = ["Hello", "How are you?", "Goodbye!"]

for msg in messages:
    print(f"Sending: {msg}")
    conn.send(msg)

    # Receive echo
    response = conn.receive(timeout=5)
    print(f"Received: {response}")

# Test JSON
print("\nSending JSON message...")
conn.send({"type": "test", "data": [1, 2, 3]})
response = conn.receive(timeout=5)
print(f"Received: {response}")

# Close connection
print("\nClosing connection...")
conn.close()
print("Done!")
