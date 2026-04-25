#!/usr/bin/env scriptling
"""Example demonstrating multicast group messaging using scriptling.net.multicast"""

import scriptling.net.multicast as mc
import scriptling.runtime as runtime

print("=== Multicast Group Messaging ===\n")

group_addr = "239.255.0.1"
group_port = 19999

# Use a WaitGroup to signal when the listener is ready before sending
ready = runtime.sync.WaitGroup("listener_ready")
ready.add(1)

def listener(addr, port):
    ready = runtime.sync.WaitGroup("listener_ready")
    group = mc.join(addr, port)
    ready.done()  # signal: socket is bound and ready to receive
    msg = group.receive(timeout=5)
    group.close()
    return msg

promise = runtime.background("listener", "listener", group_addr, group_port)
ready.wait()  # wait until listener has joined before sending

# Join as sender and send messages
group = mc.join(group_addr, group_port)
print(f"Joined multicast group {group.group_addr}:{group.port}")
print(f"Local address: {group.local_addr}")

group.send("Hello multicast group!")
print("Sent: Hello multicast group!")

group.send({"event": "heartbeat", "node": "node-1", "ts": 12345})
print("Sent JSON heartbeat message")

group.close()

# Collect the received message
msg = promise.get()
if msg:
    print(f"\nReceived from {msg['source']}: {msg['data']}")
else:
    print("\nNo message received (timeout)")

print(f"\nGroup properties:")
print(f"  group_addr: {group_addr}")
print(f"  port: {group_port}")

print("\n=== Multicast Example Complete ===")
