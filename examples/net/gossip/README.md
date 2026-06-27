# Gossip Protocol Examples

Examples demonstrating cluster membership, metadata, and messaging with `scriptling.net.gossip`, built on the [gossip](https://github.com/paularlott/gossip) library.

## Files

- `gossip_demo.py` - Single-node demo showing cluster creation, metadata, and node introspection
- `gossip_cluster.py` - Two-node cluster with broadcast messaging and handler registration
- `gossip_nodegroups.py` - Three-node cluster demonstrating metadata-criteria-based node groups
- `gossip_leader.py` - Three-node cluster with leader election and failover
- `gossip_request_reply_server.py` / `gossip_request_reply_client.py` - Request/reply across two processes; the server shows the `wait()` serve loop
- `gossip_advanced.py` - Advanced config, event handlers, node queries, and metadata change tracking

## Running

```bash
# Build the CLI first (from repo root)
task build

# Single-node demo (metadata, stats, introspection)
./bin/scriptling examples/net/gossip/gossip_demo.py

# Two-node cluster (message passing)
./bin/scriptling examples/net/gossip/gossip_cluster.py

# Node groups (metadata-based grouping)
./bin/scriptling examples/net/gossip/gossip_nodegroups.py

# Leader election and failover
./bin/scriptling examples/net/gossip/gossip_leader.py

# Request/reply messaging (run the server, then the client in another terminal)
./bin/scriptling examples/net/gossip/gossip_request_reply_server.py   # terminal 1
./bin/scriptling examples/net/gossip/gossip_request_reply_client.py   # terminal 2

# Advanced features (events, queries, config)
./bin/scriptling examples/net/gossip/gossip_advanced.py
```

## Handlers run on the script thread (call `wait()`)

Registered callbacks - `handle`, `handle_with_reply`, `on_state_change`,
`on_metadata_change`, `on_gossip_interval`, node-group `on_node_added` /
`on_node_removed`, and leader-election `on_event` - do **not** run on a
background thread. They are queued as messages and events arrive and run on your
script's thread when you call `cluster.wait()`. This keeps your handlers free of
concurrency: they never run while the rest of your script is running.

A long-running node serves events in a loop:

```python
cluster.handle(gossip.MSG_USER, on_message)
while True:
    cluster.wait(1)   # run queued handlers, blocking up to ~1s per tick
```

`wait(timeout)`:
- omitted / `None` - block until an event arrives (or the cluster stops)
- `0` - run whatever is already queued and return immediately (poll)
- `> 0` - if nothing is queued, wait up to that many seconds for the first event

It returns the number of handler callbacks processed.

> Because handlers run on the script thread, a single script cannot both block
> in `send_request()` and serve its own responder - run each node as its own
> process (see the request/reply server and client examples).

## API Overview

### create(bind_addr, ...)

Create a gossip cluster node with optional configuration:

```python
import scriptling.net.gossip as gossip

cluster = gossip.create(
    bind_addr="127.0.0.1:8000",
    tags=["web", "api"],
    encryption_key="32-byte-key-for-aes-256!!!",
    compression=True,
    transport="socket",
)
cluster.start()
```

**Parameters:**
- `bind_addr` - Address to bind to (default: `"127.0.0.1:8000"`)
- `node_id` - Unique node ID (auto-generated if empty)
- `advertise_addr` - Address advertised to peers
- `encryption_key` - AES key (16, 24, or 32 bytes)
- `tags` - List of tags for tag-based routing
- `compression` - Enable Snappy compression
- `bearer_token` - Authentication token
- `app_version` - Application version for compatibility checks
- `transport` - Transport type: `"socket"` (TCP/UDP) or `"http"` (default: `"socket"`)

**Advanced Configuration:**
- `compress_min_size` - Min message size for compression (default: 256)
- `gossip_interval` - Gossip interval duration string (default: `"5s"`)
- `gossip_max_interval` - Max gossip interval (default: `"20s"`)
- `metadata_gossip_interval` - Metadata gossip interval (default: `"500ms"`)
- `state_gossip_interval` - State exchange interval (default: `"45s"`)
- `fan_out_multiplier` - Fan-out scaling factor (default: 1.0)
- `ttl_multiplier` - TTL scaling factor (default: 1.0)
- `state_exchange_multiplier` - State exchange scaling (default: 0.8)
- `force_reliable_transport` - Force TCP for all messages (default: False)
- `prefer_ipv6` - Prefer IPv6 for DNS resolution (default: False)
- `health_check_interval` - Health check interval (default: `"2s"`)
- `suspect_timeout` - Time before marking node suspect (default: `"1.5s"`)
- `suspect_retry_interval` - Suspect retry interval (default: `"1s"`)
- `dead_node_timeout` - Time before suspect -> dead (default: `"15s"`)
- `node_cleanup_interval` - Dead node cleanup interval (default: `"20s"`)
- `node_retention_time` - How long to keep dead nodes (default: `"1h"`)
- `leaving_node_timeout` - Timeout before leaving -> dead (default: `"30s"`)
- `peer_recovery_interval` - Peer recovery check interval (default: `"30s"`)
- `insecure_skip_verify` - Skip TLS verification for HTTP (default: False)

### Cluster Methods

**Lifecycle:**
- `start()` - Start the cluster node
- `join(peers)` - Join existing cluster (string or list of addresses)
- `wait(timeout=None)` - Run queued handler callbacks on the script thread; returns the count processed (`None`=block until an event, `0`=poll, `>0`=wait up to N seconds)
- `leave()` - Gracefully leave the cluster
- `stop()` - Stop and clean up

**Messaging:**
- `send(msg_type, data, reliable=False)` - Broadcast to the cluster
- `send_tagged(tag, msg_type, data, reliable=False)` - Send to nodes with matching tag
- `send_to(node_id, msg_type, data, reliable=False)` - Send directly to a specific node
- `send_request(node_id, msg_type, data)` - Send a request and wait for a reply
- `handle(msg_type, handler_fn)` - Register a message handler (msg_type >= 128)
- `handle_with_reply(msg_type, handler_fn)` - Register a request/reply handler
- `unhandle(msg_type)` - Remove a previously registered handler (returns bool)

**Node Info:**
- `nodes()` - All known nodes
- `alive_nodes()` - Alive nodes only
- `nodes_by_tag(tag)` - Get nodes with a specific tag
- `get_node(node_id)` - Get a specific node by ID (returns dict or None)
- `local_node()` - Local node info dict
- `num_nodes()` / `num_alive()` / `num_suspect()` / `num_dead()` - Node counts
- `node_id()` - Local node's UUID
- `is_local(node_id)` - Check if a node ID is the local node
- `candidates()` - Random subset of nodes for gossiping

**Metadata:**
- `set_metadata(key, value)` - Set metadata (auto-gossiped)
- `get_metadata(key)` - Get value (returns string or None)
- `all_metadata()` - Get all metadata as dict
- `delete_metadata(key)` - Delete a key

**Events:**
- `on_state_change(handler_fn)` - Register node state change handler `fn(node_id, new_state)`
- `on_metadata_change(handler_fn)` - Register metadata change handler `fn(node_dict)`
- `on_gossip_interval(handler_fn)` - Register periodic handler `fn()` called every gossip interval

**Node Groups:**
- `create_node_group(criteria, on_node_added=None, on_node_removed=None)` - Create a metadata-criteria-based node group

**Leader Election:**
- `create_leader_election(...)` - Create a leader election manager

### Node Groups

Node groups track nodes matching metadata criteria with automatic membership management:

```python
# Create a group for nodes with role=worker in us-east
workers = cluster.create_node_group(
    criteria={"role": "worker", "zone": "us-east"},
    on_node_added=lambda node: print(f"Worker joined: {node['id']}"),
    on_node_removed=lambda node: print(f"Worker left: {node['id']}"),
)

# Query the group
workers.nodes()        # list of matching nodes
workers.count()        # number of nodes
workers.contains(id)   # check if node is in group

# Send to all peers in the group
workers.send_to_peers(128, {"task": "process"})

# Clean up
workers.close()
```

**Criteria matching:**
- Exact match: `{"role": "worker"}` - value must equal "worker"
- Any value: `{"role": "*"}` - key must exist, any value
- Contains: `{"role": "~work"}` - value must contain "work"

### Leader Election

Leader election with configurable quorum and optional metadata filtering:

```python
election = cluster.create_leader_election(
    check_interval="1s",
    leader_timeout="3s",
    quorum_percentage=60,
    metadata_criteria={"role": "leader-eligible"},
)

election.on_event("became_leader", lambda evt, node_id: print("I am the leader!"))
election.on_event("stepped_down", lambda evt, node_id: print("No longer leader"))
election.on_event("elected", lambda evt, node_id: print(f"New leader: {node_id}"))
election.on_event("lost", lambda evt, node_id: print("Leader lost"))

election.start()

if election.is_leader():
    election.send_to_peers(128, "leader message")

election.stop()
```

**Parameters:**
- `check_interval` - Duration between checks (default: `"1s"`)
- `leader_timeout` - Duration without heartbeat before leader lost (default: `"3s"`)
- `heartbeat_msg_type` - Message type for heartbeats (default: 65, reserved range)
- `quorum_percentage` - Percentage of nodes for quorum 1-100 (default: 60)
- `metadata_criteria` - Optional dict to limit eligible nodes

**Methods:**
- `start()` / `stop()` - Start/stop election process
- `is_leader()` - Check if local node is leader
- `has_leader()` - Check if any leader is elected
- `get_leader_id()` - Get leader's node ID (or None)
- `send_to_peers(msg_type, data, reliable=False)` - Send to eligible peers
- `on_event(event_type, handler_fn)` - Register event handler

**Event types:** `"elected"`, `"lost"`, `"became_leader"`, `"stepped_down"`

### Request/Reply Messaging

Send a message and wait for a response from a specific node:

```python
# --- Responder process ---
cluster.handle_with_reply(200, lambda msg: {"echo": msg["payload"]})
while True:
    cluster.wait(1)   # serve requests on the script thread

# --- Requester process ---
reply = cluster.send_request(target_node_id, 200, "hello")
# reply == {"echo": "hello"}
```

The responder must run its own `wait()` loop (typically in its own process). A
single script cannot both block in `send_request()` and serve the matching
`handle_with_reply()` handler, because both need the one script thread.

### Message Types

User message types start at 128 (`gossip.MSG_USER`). The handler receives a dict:

```python
cluster.handle(gossip.MSG_USER, lambda msg: print(msg["payload"]))

# msg dict contains:
#   "type"    - message type (int)
#   "sender"  - dict with id, addr, state, metadata, tags
#   "payload" - decoded message payload
```

### Constants

- `gossip.MSG_USER` (128) - Starting message type for user-defined messages
