# Multicast Example

Example demonstrating UDP multicast group messaging with `scriptling.net.multicast`.

## Files

- `multicast_demo.py` - Join a multicast group, send messages, and inspect group properties

## Running

```bash
# Build the CLI first (from repo root)
task build

./bin/scriptling examples/net/multicast/multicast_demo.py
```

## API Overview

### join(group_addr, port, interface="")

Join a multicast group. Returns a group object.

```python
import scriptling.net.multicast as mc

group = mc.join("239.255.0.1", 9999)
group.send("Hello group!")
msg = group.receive(timeout=5)  # returns dict with "data" and "source", or None
group.close()
```

### Group Object

- `send(message)` - Send a string or dict to the group (dicts are JSON-encoded)
- `receive(timeout=30)` - Receive a message, returns dict with `data` and `source`, or `None` on timeout
- `close()` - Leave the group and close the connection
- `group_addr` - Multicast group address
- `port` - Port number
- `local_addr` - Local bound address

### Notes

- The group address must be a valid multicast address (224.0.0.0 - 239.255.255.255)
- To receive messages sent by yourself, you need another process joined to the same group
- Use the `interface` parameter to bind to a specific network interface
