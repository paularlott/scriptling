#!/usr/bin/env scriptling
"""Request/reply requester node (run gossip_request_reply_server.py first).

Joins the responder and issues blocking send_request() calls. A requester has no
handlers of its own, so it does not need a wait() loop — send_request() blocks
until the reply arrives.

Run each node as its own process:
    scriptling gossip_request_reply_server.py     # terminal 1
    scriptling gossip_request_reply_client.py     # terminal 2
"""

import scriptling.net.gossip as gossip
import time

print("=== Gossip Request/Reply Client ===\n")

requester = gossip.create(bind_addr="127.0.0.1:19302")
requester.start()
requester.set_metadata("name", "requester")

# Join the responder and wait for the cluster to converge.
requester.join(["127.0.0.1:19301"])
time.sleep(1)
print(f"Cluster formed: {requester.num_alive()} alive node(s)\n")

# Find the responder by metadata.
responder_id = None
for n in requester.alive_nodes():
    if not requester.is_local(n["id"]):
        responder_id = n["id"]
        break

if responder_id == None:
    print("No responder found - start gossip_request_reply_server.py first")
else:
    print("Sending echo request...")
    reply = requester.send_request(responder_id, 200, "Hello!")
    print(f"  Reply: {reply}")

    print("\nSending dict echo request...")
    reply = requester.send_request(responder_id, 200, {"key": "value", "count": 42})
    print(f"  Reply: {reply}")

    print("\nSending double request: value=21...")
    reply = requester.send_request(responder_id, 201, {"value": 21})
    print(f"  Reply: {reply}")

requester.stop()
print("\nClient done.")
