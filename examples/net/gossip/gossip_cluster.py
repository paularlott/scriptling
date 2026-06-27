#!/usr/bin/env scriptling
"""Example demonstrating a two-node gossip cluster with message passing.

This script creates two cluster nodes, joins them together, and
demonstrates broadcast messaging and node state tracking.
"""

import scriptling.net.gossip as gossip
import scriptling.runtime as runtime
import time

print("=== Gossip Two-Node Cluster ===\n")

# Create node A on a fixed port
nodeA = gossip.create(bind_addr="127.0.0.1:19001")
nodeA.start()
nodeA.set_metadata("name", "node-A")
print(f"Node A started on 127.0.0.1:19001")

# Create node B on a different fixed port
nodeB = gossip.create(bind_addr="127.0.0.1:19002")
nodeB.start()
nodeB.set_metadata("name", "node-B")
print(f"Node B started on 127.0.0.1:19002")

# Join B to A
nodeB.join(["127.0.0.1:19001"])
print(f"\nNode B joined Node A")

# Wait for cluster to converge
time.sleep(1)

# Check cluster membership
print(f"\nNode A sees {nodeA.num_alive()} alive node(s)")
print(f"Node B sees {nodeB.num_alive()} alive node(s)")

for n in nodeA.alive_nodes():
    name = n["metadata"]["name"] if "name" in n["metadata"] else "unknown"
    print(f"  {n['id'][:16]}... state={n['state']} name={name}")

# Register message handler on B using a sync queue
messages = runtime.sync.Queue("msgs", maxsize=10)

def on_message(msg):
    messages.put(msg)

nodeB.handle(gossip.MSG_USER, on_message)
print("\nRegistered handler on Node B")

# Broadcast from A
print("\nNode A broadcasting...")
nodeA.send(gossip.MSG_USER, "Hello from Node A!")
# Sleep allows gossip propagation before we check the queue
time.sleep(2)

if messages.size() > 0:
    msg = messages.get()
    print(f"  Node B received: payload={msg['payload']}")
    print(f"  Sender name: {msg['sender']['metadata']['name']}")
else:
    print("  (no message received within 2s)")

# Send a dict payload
nodeA.send(gossip.MSG_USER, {"event": "test", "value": 42})
time.sleep(2)

if messages.size() > 0:
    msg = messages.get()
    print(f"  Node B received dict: {msg['payload']}")
else:
    print("  (no message received within 2s)")

# Clean up
nodeA.stop()
nodeB.stop()
print("\nBoth nodes stopped.")
print("\n=== Gossip Two-Node Demo Complete ===")
