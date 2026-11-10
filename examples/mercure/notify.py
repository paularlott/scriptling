# Publish one update: a message for listener.py
#
# Run with: scriptling examples/mercure/notify.py "Hello there"

import os
import sys
import scriptling.mercure as mercure

URL = os.getenv("MERCURE_URL", "http://localhost:3000/.well-known/mercure")
KEY = os.getenv("MERCURE_JWT_KEY", "!ChangeThisMercureHubJWTSecretKey!")
TOPIC = os.getenv("MERCURE_TOPIC", "https://example.com/notifications")

message = " ".join(sys.argv[1:]) or "Hello from scriptling"
with mercure.Client(URL, key=KEY, publish=[TOPIC]) as hub:
    print(f"Published {hub.publish(TOPIC, message)}")
