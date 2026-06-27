#!/usr/bin/env scriptling
"""Request/reply responder node (run this, then gossip_request_reply_client.py).

Registers handle_with_reply handlers and then serves them with a wait() loop.

Handlers run on the script thread when you call cluster.wait(); they never run
concurrently with the rest of your script. A long-running node therefore loops:

    while True:
        cluster.wait(1)   # process events, ~1s ticks

Run each node as its own process:
    scriptling gossip_request_reply_server.py     # terminal 1
    scriptling gossip_request_reply_client.py     # terminal 2
"""

import scriptling.net.gossip as gossip

print("=== Gossip Request/Reply Responder ===\n")

responder = gossip.create(bind_addr="127.0.0.1:19301")
responder.start()
responder.set_metadata("name", "responder")
print(f"Responder listening on 127.0.0.1:19301 ({responder.node_id()[:16]}...)")


# Echo service (msg type 200): returns the payload back to the sender.
def echo_handler(msg):
    return {"echo": msg["payload"], "from": "responder"}


responder.handle_with_reply(200, echo_handler)


# Math service (msg type 201): doubles a number.
def double_handler(msg):
    payload = msg["payload"]
    if type(payload) == "DICT" and "value" in payload:
        return {"result": payload["value"] * 2}
    return {"error": "expected {value: number}"}


responder.handle_with_reply(201, double_handler)
print("Registered echo (200) and double (201) handlers")
print("\nServing requests (Ctrl+C to stop)...\n")

# Serve loop: process handler callbacks on this thread until interrupted.
# wait(1) blocks up to 1s for the next event, runs it, and returns the count.
while True:
    handled = responder.wait(1)
    if handled > 0:
        print(f"  processed {handled} request(s)")
