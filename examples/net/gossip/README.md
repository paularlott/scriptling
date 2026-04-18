# Gossip Protocol Examples

Examples demonstrating cluster membership, metadata, and messaging with `scriptling.net.gossip`, built on the [gossip](https://github.com/paularlott/gossip) library.

## Files

- `gossip_demo.py` - Single-node demo showing cluster creation, metadata, and node introspection
- `gossip_cluster.py` - Two-node cluster with broadcast messaging and handler registration

## Running

```bash
# Build the CLI first (from repo root)
task build

# Single-node demo (metadata, stats, introspection)
./bin/scriptling examples/net/gossip/gossip_demo.py

# Two-node cluster (message passing)
./bin/scriptling examples/net/gossip/gossip_cluster.py
```

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

### Cluster Methods

**Lifecycle:**
- `start()` - Start the cluster node
- `join(peers)` - Join existing cluster (string or list of addresses)
- `leave()` - Gracefully leave the cluster
- `stop()` - Stop and clean up

**Messaging:**
- `send(msg_type, data, reliable=False)` - Broadcast to the cluster
- `send_tagged(tag, msg_type, data, reliable=False)` - Send to nodes with matching tag
- `send_to(node_id, msg_type, data, reliable=False)` - Send directly to a specific node
- `handle(msg_type, handler_fn)` - Register a message handler (msg_type >= 128)

**Node Info:**
- `nodes()` - All known nodes
- `alive_nodes()` - Alive nodes only
- `local_node()` - Local node info dict
- `num_nodes()` / `num_alive()` - Node counts
- `node_id()` - Local node's UUID

**Metadata:**
- `set_metadata(key, value)` - Set metadata (auto-gossiped)
- `get_metadata(key)` - Get value (returns string or None)
- `all_metadata()` - Get all metadata as dict
- `delete_metadata(key)` - Delete a key

**Events:**
- `on_state_change(handler_fn)` - Register node state change handler

### Message Types

User message types start at 128 (`gossip.MSG_USER`). The handler receives a dict:

```python
cluster.handle(gossip.MSG_USER, lambda msg: print(msg["payload"]))

# msg dict contains:
#   "type"    - message type (int)
#   "sender"  - dict with id, addr, state, metadata
#   "payload" - decoded message payload
```

### Constants

- `gossip.MSG_USER` (128) - Starting message type for user-defined messages
