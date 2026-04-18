# Unicast Examples

Examples demonstrating UDP and TCP point-to-point messaging with `scriptling.net.unicast`.

## Files

- `tcp_echo.py` - TCP echo server and client exchanging messages and JSON payloads
- `udp_ping_pong.py` - UDP ping-pong with batch message sending

## Running

```bash
# Build the CLI first (from repo root)
task build

# TCP echo
./bin/scriptling examples/net/unicast/tcp_echo.py

# UDP ping-pong
./bin/scriptling examples/net/unicast/udp_ping_pong.py
```

## API Overview

### connect(host, port, protocol="udp", timeout=10)

Connect to a remote host. Returns a connection object.

```python
import scriptling.net.unicast as uc

conn = uc.connect("192.168.1.1", 8080, protocol="tcp")
conn.send("Hello!")
msg = conn.receive(timeout=5)  # returns dict with "data" and "source" keys
conn.close()
```

### listen(host, port, protocol="tcp")

Listen for incoming connections. Returns a listener object.

**TCP listener** has `accept(timeout)`, `close()`, `addr`:

```python
server = uc.listen("0.0.0.0", 8080, protocol="tcp")
conn = server.accept(timeout=60)
msg = conn.receive()
conn.send("Echo: " + msg["data"])
conn.close()
server.close()
```

**UDP listener** has `receive(timeout)`, `send_to(addr, msg)`, `close()`, `addr`:

```python
server = uc.listen("0.0.0.0", 8080, protocol="udp")
msg = server.receive(timeout=10)
server.send_to(msg["source"], "reply")
server.close()
```

### Connection Object Methods

- `send(message)` - Send a string or dict (dicts are JSON-encoded)
- `receive(timeout=30)` - Receive a message, returns dict with `data` and `source`, or `None` on timeout
- `close()` - Close the connection
- `connected()` - Check if connection is open
- `local_addr` - Local address string
- `remote_addr` - Remote address string
