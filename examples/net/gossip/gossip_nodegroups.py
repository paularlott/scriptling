#!/usr/bin/env scriptling
"""Example demonstrating metadata-criteria-based node groups.

Creates three nodes with different metadata and uses create_node_group()
to automatically track subsets of nodes matching specific criteria.
"""

import scriptling.net.gossip as gossip
import time

print("=== Gossip Node Groups Demo ===\n")

# Create node A: web server in us-east
nodeA = gossip.create(bind_addr="127.0.0.1:19101")
nodeA.start()
nodeA.set_metadata("role", "web")
nodeA.set_metadata("zone", "us-east")
print("Node A started (role=web, zone=us-east)")

# Create node B: api server in us-east
nodeB = gossip.create(bind_addr="127.0.0.1:19102")
nodeB.start()
nodeB.set_metadata("role", "api")
nodeB.set_metadata("zone", "us-east")
print("Node B started (role=api, zone=us-east)")

# Create node C: api server in eu-west
nodeC = gossip.create(bind_addr="127.0.0.1:19103")
nodeC.start()
nodeC.set_metadata("role", "api")
nodeC.set_metadata("zone", "eu-west")
print("Node C started (role=api, zone=eu-west)")

# Join B and C to A
nodeB.join(["127.0.0.1:19101"])
nodeC.join(["127.0.0.1:19101"])
time.sleep(1)
print(f"\nCluster formed: {nodeA.num_alive()} alive nodes")

# Group: all API servers
api_group = nodeA.create_node_group(criteria={"role": "api"})
print(f"\nAPI servers group: {api_group.count()} node(s)")
for n in api_group.nodes():
    print(f"  {n['metadata']['zone']}: {n['id'][:16]}...")

# Group: us-east nodes (wildcard match on role)
east_group = nodeA.create_node_group(criteria={"zone": "us-east"})
print(f"\nus-east group: {east_group.count()} node(s)")
for n in east_group.nodes():
    print(f"  role={n['metadata']['role']}: {n['id'][:16]}...")

# Group: exact match (role=api AND zone=us-east)
api_east = nodeA.create_node_group(criteria={"role": "api", "zone": "us-east"})
print(f"\nAPI+us-east group: {api_east.count()} node(s)")

# Check membership
local_id = nodeA.node_id()
print(f"\nNode A in api_group? {api_group.contains(local_id)}")
print(f"Node A in east_group? {east_group.contains(local_id)}")

# contains() also works with remote node IDs
b_nodes = nodeB.alive_nodes()
for n in b_nodes:
    if n["metadata"]["role"] == "api":
        print(f"Node B sees API node {n['id'][:16]}... in api_group? {api_group.contains(n['id'])}")

# Clean up
api_group.close()
east_group.close()
api_east.close()
nodeA.stop()
nodeB.stop()
nodeC.stop()
print("\nAll nodes and groups stopped.")
print("\n=== Demo Complete ===")
