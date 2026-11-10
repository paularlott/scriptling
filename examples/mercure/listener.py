# A long-running subscriber: prints every update until stopped (Ctrl+C)
#
# Run with:  scriptling examples/mercure/listener.py
# then send: scriptling examples/mercure/notify.py "Hello"
#
# It runs for as long as you leave it. The client renews its token before it
# expires, and the subscription reconnects by itself (a hub restart, a dropped
# network, a token reaching its end), resuming from its last event ID, so
# nothing the hub kept is missed.

import os
import scriptling.mercure as mercure

URL = os.getenv("MERCURE_URL", "http://localhost:3000/.well-known/mercure")
KEY = os.getenv("MERCURE_JWT_KEY", "!ChangeThisMercureHubJWTSecretKey!")
TOPIC = os.getenv("MERCURE_TOPIC", "https://example.com/notifications")

hub = mercure.Client(URL, key=KEY, subscribe=[TOPIC])
print(f"Listening on {TOPIC} at {URL}")

for event in hub.subscribe(TOPIC):
    print(f"[{event.id}] {event.data}")
