#!/usr/bin/env scriptling
"""Example demonstrating advanced gossip configuration and event handlers.

Creates a cluster with custom timing parameters and demonstrates
on_metadata_change, on_gossip_interval, is_local, and candidates.
"""

import scriptling.net.gossip as gossip
import scriptling.runtime as runtime
import time

print("=== Gossip Advanced Features Demo ===\n")

# Create cluster with custom config
cluster = gossip.create(
    bind_addr="127.0.0.1:19401",
    tags=["web", "api"],
    gossip_interval="2s",
    health_check_interval="3s",
    node_retention_time="30m",
)
cluster.start()
cluster.set_metadata("name", "node-A")
cluster.set_metadata("role", "web")
print("Node A started with custom config")

# Create a second node
nodeB = gossip.create(bind_addr="127.0.0.1:19402")
nodeB.start()
nodeB.set_metadata("name", "node-B")
nodeB.set_metadata("role", "api")
print("Node B started")

# Join and wait
nodeB.join(["127.0.0.1:19401"])
time.sleep(1)
print(f"Cluster formed: {cluster.num_alive()} alive nodes\n")

# Node state counts
print(f"Node counts: alive={cluster.num_alive()}, suspect={cluster.num_suspect()}, dead={cluster.num_dead()}, total={cluster.num_nodes()}")

# is_local check
a_id = cluster.node_id()
b_nodes = cluster.alive_nodes()
for n in b_nodes:
    local = cluster.is_local(n["id"])
    name = n["metadata"]["name"] if "name" in n["metadata"] else "?"
    print(f"  {name}: is_local={local}")

# candidates() - random gossip subset
cands = cluster.candidates()
print(f"\nGossip candidates: {len(cands)} node(s)")
for c in cands:
    name = c["metadata"]["name"] if "name" in c["metadata"] else "?"
    print(f"  {name}: {c['id'][:16]}...")

# nodes_by_tag
web_nodes = cluster.nodes_by_tag("web")
api_nodes = cluster.nodes_by_tag("api")
print(f"\nNodes with tag 'web': {len(web_nodes)}")
print(f"Nodes with tag 'api': {len(api_nodes)}")

# get_node - lookup by ID
if len(b_nodes) > 0:
    first_remote = None
    for n in b_nodes:
        if not cluster.is_local(n["id"]):
            first_remote = n["id"]
            break
    if first_remote != None:
        found = cluster.get_node(first_remote)
        print(f"\nget_node({first_remote[:16]}...): name={found['metadata']['name']}")

missing = cluster.get_node("00000000-0000-0000-0000-000000000000")
print(f"get_node(unknown): {missing}")

# on_metadata_change - fires when remote nodes change metadata
metadata_changes = runtime.sync.Queue("meta_changes", maxsize=10)

def on_metadata_change(node):
    name = node["metadata"]["name"] if "name" in node["metadata"] else "?"
    metadata_changes.put({"node": name, "metadata": node["metadata"]})

cluster.on_metadata_change(on_metadata_change)
print("\nRegistered metadata change handler")

# Trigger a metadata change on node B
nodeB.set_metadata("version", "2.0")
print("Node B updated metadata (version=2.0)")
time.sleep(2)

if metadata_changes.size() > 0:
    change = metadata_changes.get()
    print(f"  Detected change on '{change['node']}': {change['metadata']}")
else:
    print("  (metadata change not propagated yet)")

# unhandle - register then remove a handler
cluster.handle(200, lambda msg: None)
removed = cluster.unhandle(200)
print(f"\nunhandle(200): removed={removed}")
removed_again = cluster.unhandle(200)
print(f"unhandle(200) again: removed={removed_again}")

# Clean up
nodeB.stop()
cluster.stop()
print("\nAll nodes stopped.")
print("\n=== Demo Complete ===")
