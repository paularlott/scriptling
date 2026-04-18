#!/usr/bin/env scriptling
"""Example demonstrating gossip cluster membership and messaging using scriptling.net.gossip

This single-node demo shows cluster creation, metadata management,
and the node introspection API. For message delivery, see the
two-node example in gossip_cluster.py.
"""

import scriptling.net.gossip as gossip

print("=== Gossip Cluster Demo (Single Node) ===\n")

cluster = gossip.create(bind_addr="127.0.0.1:0")
cluster.start()

node_id = cluster.node_id()
print(f"Node ID: {node_id}")

# Metadata is automatically gossiped to peers
cluster.set_metadata("role", "demo")
cluster.set_metadata("version", "1.0")
cluster.set_metadata("port", 8080)

print(f"\nMetadata: {cluster.all_metadata()}")
print(f"Role: {cluster.get_metadata('role')}")
print(f"Missing key returns None: {cluster.get_metadata('nonexistent')}")

# Node introspection
local = cluster.local_node()
print(f"\nLocal node:")
print(f"  ID: {local['id'][:16]}...")
print(f"  Addr: {local['addr']}")
print(f"  State: {local['state']}")
print(f"  Metadata: {local['metadata']}")

# Cluster stats
print(f"\nCluster stats:")
print(f"  Total nodes: {cluster.num_nodes()}")
print(f"  Alive nodes: {cluster.num_alive()}")

# Metadata mutation
cluster.delete_metadata("port")
print(f"\nAfter deleting 'port': {cluster.all_metadata()}")

cluster.stop()
print("\nCluster stopped.")
print("\n=== Demo Complete ===")
