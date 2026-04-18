#!/usr/bin/env scriptling
"""Example demonstrating multicast group messaging using scriptling.net.multicast"""

import scriptling.net.multicast as mc

print("=== Multicast Group Messaging ===\n")

# Join a multicast group
group_addr = "239.255.0.1"
group_port = 19999

group = mc.join(group_addr, group_port)
print(f"Joined multicast group {group.group_addr}:{group.port}")
print(f"Local address: {group.local_addr}")

# Send a message to the group
group.send("Hello multicast group!")
print("Sent: Hello multicast group!")

# Try to receive (with short timeout - will likely timeout
# since we need another listener, but demonstrates the API)
msg = group.receive(timeout=1)
if msg:
    print(f"Received from {msg['source']}: {msg['data']}")
else:
    print("No message received (timeout - expected with single node)")

# Send a dict (auto-encoded as JSON)
group.send({"event": "heartbeat", "node": "node-1", "ts": 12345})
print("Sent JSON heartbeat message")

print(f"\nGroup properties:")
print(f"  group_addr: {group.group_addr}")
print(f"  port: {group.port}")
print(f"  local_addr: {group.local_addr}")

group.close()
print("\nGroup closed.")

print("\n=== Multicast Example Complete ===")
