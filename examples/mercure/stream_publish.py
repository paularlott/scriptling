# Publish many updates quickly
#
# By default each publish is a POST of its own, which every Mercure hub takes.
# A gossiper hub also takes a streaming publish: every update over one long
# request, without waiting for each answer. stream=True asks for it;
# stream="auto" streams to a gossiper hub and falls back to a POST per update
# with any other. The stream reopens by itself with a fresh token as each one
# expires, losing nothing.
#
# Run with: scriptling examples/mercure/stream_publish.py

import os
import time
import scriptling.mercure as mercure

URL = os.getenv("MERCURE_URL", "http://localhost:3000/.well-known/mercure")
KEY = os.getenv("MERCURE_JWT_KEY", "!ChangeThisMercureHubJWTSecretKey!")
COUNT = int(os.getenv("MERCURE_COUNT", "2000"))
TOPIC = "https://example.com/ticks"

updates = [{"topic": TOPIC, "data": {"n": i}} for i in range(COUNT)]


def publish_all(stream):
    with mercure.Client(URL, key=KEY, stream=stream) as hub:
        sub = hub.subscribe(TOPIC, buffer=COUNT + 10)
        start = time.time()
        ids = hub.publish_many(updates)
        took = time.time() - start
        transport = hub.transport

        # Every update arrives, in the order published.
        received = 0
        for event in sub:
            if event.json()["n"] != received:
                print(f"  out of order at {received}")
                break
            received += 1
            if received == COUNT:
                break
    rate = int(COUNT / took) if took > 0 else COUNT
    print(f"{transport:>6}: {len(ids)} published in {took:.2f}s ({rate}/s), {received} received")


publish_all(False)   # a POST per update
publish_all("auto")  # a stream when the hub is gossiper
