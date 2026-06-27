#!/usr/bin/env scriptling
"""Example demonstrating leader election with gossip.

Creates three nodes and uses create_leader_election() to elect a leader.
The node with the lowest ID becomes the leader. Demonstrates event
handlers and leader state queries.
"""

import scriptling.net.gossip as gossip
import scriptling.runtime as runtime
import time

print("=== Gossip Leader Election Demo ===\n")

events = runtime.sync.Queue("leader_events", maxsize=20)

# Create three nodes
nodeA = gossip.create(bind_addr="127.0.0.1:19201")
nodeA.start()
nodeA.set_metadata("name", "node-A")

nodeB = gossip.create(bind_addr="127.0.0.1:19202")
nodeB.start()
nodeB.set_metadata("name", "node-B")

nodeC = gossip.create(bind_addr="127.0.0.1:19203")
nodeC.start()
nodeC.set_metadata("name", "node-C")

print(f"Node A: {nodeA.node_id()[:16]}...")
print(f"Node B: {nodeB.node_id()[:16]}...")
print(f"Node C: {nodeC.node_id()[:16]}...")

# Join cluster
nodeB.join(["127.0.0.1:19201"])
nodeC.join(["127.0.0.1:19201"])
time.sleep(1)
print(f"\nCluster formed: {nodeA.num_alive()} alive nodes")

# Set up leader election on node A with event tracking
election = nodeA.create_leader_election(
    check_interval="500ms",
    leader_timeout="2s",
    quorum_percentage=60,
)

def on_election_event(event_type, node_id):
    events.put({"event": event_type, "node_id": node_id})

election.on_event("elected", on_election_event)
election.on_event("became_leader", on_election_event)
election.on_event("stepped_down", on_election_event)
election.on_event("lost", on_election_event)

print("\nStarting leader election...")
election.start()
time.sleep(1)

# Check leader state
print(f"\nHas leader: {election.has_leader()}")
print(f"Is leader (node A): {election.is_leader()}")

leader_id = election.get_leader_id()
if leader_id != None:
    print(f"Leader ID: {leader_id[:16]}...")
    leader_node = nodeA.get_node(leader_id)
    if leader_node != None:
        print(f"Leader name: {leader_node['metadata']['name']}")

# Process any queued events
while events.size() > 0:
    evt = events.get()
    print(f"  Event: {evt['event']} -> {evt['node_id'][:16]}...")

# Demonstrate leader-scoped messaging
if election.is_leader():
    print("\nNode A is the leader, can send to eligible peers")
    election.send_to_peers(gossip.MSG_USER, {"from": "leader"})

# Now test leader failover: stop the current leader
if leader_id != None:
    print(f"\n--- Leader Failover Test ---")
    if election.is_leader():
        print("Stopping node A (current leader)...")
        election.stop()
        nodeA.stop()
    elif nodeA.get_node(leader_id) != None and nodeA.get_node(leader_id)["metadata"]["name"] == "node-B":
        print("Stopping node B (current leader)...")
        nodeB.stop()
    else:
        print("Stopping node C (current leader)...")
        nodeC.stop()

    time.sleep(2)

    # Check if a new leader was elected on remaining nodes
    # (Only if node A is still alive)
    if nodeA.node_id() != None and nodeA.num_alive() != None:
        alive = nodeA.num_alive()
        print(f"Remaining alive nodes: {alive}")
        if election.has_leader():
            new_leader = election.get_leader_id()
            if new_leader != None:
                print(f"New leader elected: {new_leader[:16]}...")

# Clean up remaining
election.stop()
if nodeA.num_alive() != None:
    nodeA.stop()
nodeB.stop()
nodeC.stop()
print("\nAll nodes stopped.")
print("\n=== Demo Complete ===")
