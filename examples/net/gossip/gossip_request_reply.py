#!/usr/bin/env scriptling
"""Example demonstrating request/reply messaging between gossip nodes.

Creates two nodes. One registers a handle_with_reply handler that
processes requests and returns responses. The other uses send_request
to send a message and wait for the reply.
"""

import scriptling.net.gossip as gossip
import scriptling.runtime as runtime
import time

print("=== Gossip Request/Reply Demo ===\n")

# Create responder node
responder = gossip.create(bind_addr="127.0.0.1:19301")
responder.start()
responder.set_metadata("name", "responder")
print(f"Responder started: {responder.node_id()[:16]}...")

# Create requester node
requester = gossip.create(bind_addr="127.0.0.1:19302")
requester.start()
requester.set_metadata("name", "requester")
print(f"Requester started: {requester.node_id()[:16]}...")

# Join cluster
requester.join(["127.0.0.1:19301"])
time.sleep(1)
print(f"Cluster formed: {requester.num_alive()} alive nodes\n")

# Register a reply handler: echo service (msg type 200)
def echo_handler(msg):
    return {"echo": msg["payload"], "from": "responder"}

responder.handle_with_reply(200, echo_handler)
print("Registered echo handler on responder (msg type 200)")

# Register a math handler: doubles a number (msg type 201)
def double_handler(msg):
    payload = msg["payload"]
    if type(payload) == "DICT" and "value" in payload:
        val = payload["value"]
        return {"result": val * 2}
    return {"error": "expected {value: number}"}

responder.handle_with_reply(201, double_handler)
print("Registered double handler on responder (msg type 201)")

# Give handlers time to register and propagate
time.sleep(1)

# Send echo request
responder_id = responder.node_id()
print(f"\nSending echo request to responder...")
reply = requester.send_request(responder_id, 200, "Hello!")
print(f"  Sent: 'Hello!'")
print(f"  Reply: {reply}")

# Send echo with a dict
print(f"\nSending dict echo request...")
reply = requester.send_request(responder_id, 200, {"key": "value", "count": 42})
print(f"  Sent: {{'key': 'value', 'count': 42}}")
print(f"  Reply: {reply}")

# Send double request
print(f"\nSending double request: value=21...")
reply = requester.send_request(responder_id, 201, {"value": 21})
print(f"  Reply: {reply}")

# Send double with larger number
print(f"\nSending double request: value=256...")
reply = requester.send_request(responder_id, 201, {"value": 256})
print(f"  Reply: {reply}")

# Demonstrate that regular handle() still works alongside handle_with_reply
messages = runtime.sync.Queue("msgs", maxsize=10)
def on_broadcast(msg):
    messages.put(msg["payload"])

responder.handle(gossip.MSG_USER, on_broadcast)
requester.send(gossip.MSG_USER, "broadcast still works")
time.sleep(2)
if messages.size() > 0:
    print(f"\nBroadcast received on responder: '{messages.get()}'")

# Clean up
requester.stop()
responder.stop()
print("\nBoth nodes stopped.")
print("\n=== Demo Complete ===")
