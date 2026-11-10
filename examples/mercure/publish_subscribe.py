# Publish and subscribe with scriptling.mercure
#
# Works with any Mercure hub: the official one, FrankenPHP's, or gossiper.
# Run with: scriptling examples/mercure/publish_subscribe.py
#
# MERCURE_URL     the hub (default http://localhost:3000/.well-known/mercure)
# MERCURE_JWT_KEY the hub's JWT key (HS256)

import os
import scriptling.mercure as mercure

URL = os.getenv("MERCURE_URL", "http://localhost:3000/.well-known/mercure")
KEY = os.getenv("MERCURE_JWT_KEY", "!ChangeThisMercureHubJWTSecretKey!")

# Given the hub's key, the client makes its own tokens and renews them before
# they expire. Without publish= or subscribe= it may use every topic.
with mercure.Client(URL, key=KEY) as hub:
    print(f"Hub: {hub.url} (publishing over {hub.transport})")

    # Subscribe to every book: a URL pattern matches each book's topic.
    sub = hub.subscribe(match_urlpattern="https://example.com/books/:id")

    # Dicts (and lists, numbers) are sent as JSON; strings as they are.
    first = hub.publish("https://example.com/books/1", {"title": "Dune", "year": 1965}, type="book")
    hub.publish("https://example.com/books/2", {"title": "Hyperion", "year": 1989}, type="book")
    hub.publish("https://example.com/books/3", "a plain text update")
    print(f"Published, first event ID: {first}")

    for _ in range(3):
        event = sub.receive(timeout=10)
        if event is None:
            print("No event within 10 seconds")
            break
        if event.type == "book":
            book = event.json()
            print(f"  {event.id}: {book['title']} ({book['year']})")
        else:
            print(f"  {event.id}: {event.data}")

    # Private updates reach only subscribers allowed the topic. This client's
    # tokens allow Alice's inbox and nothing else.
    reader = mercure.Client(URL, key=KEY, subscribe=["https://example.com/inbox/alice"])
    inbox = reader.subscribe("https://example.com/inbox/alice")
    hub.publish("https://example.com/inbox/alice", "for Alice only", private=True)
    print(f"Alice's inbox: {inbox.receive(timeout=10).data}")
    reader.close()

    # A new subscription can resume after an event the hub still holds in its
    # history.
    later = hub.subscribe(match_urlpattern="https://example.com/books/:id", last_event_id=first)
    event = later.receive(timeout=5)
    if event is None:
        print("This hub keeps no history to resume from")
    else:
        print(f"Resumed after {first}: next was {event.id}")

print("Done")
