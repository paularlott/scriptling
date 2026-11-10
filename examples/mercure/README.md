# Mercure Examples

Examples for `scriptling.mercure`, a client for [Mercure](https://mercure.rocks) hubs: publish updates to
topics and subscribe to them over Server-Sent Events.

## Files

- `publish_subscribe.py` - Publish JSON and text updates, subscribe by URL pattern, private updates,
  resuming from a last event ID
- `listener.py` - A long-running subscriber that prints every update, for as long as it runs
- `notify.py` - Publish one update for `listener.py`
- `stream_publish.py` - Publish many updates with `publish_many()`, a POST per update and then over a
  stream (gossiper)

## Tokens

Give the client the hub's key and it makes its own tokens, renewing each before it expires. Subscriptions
and publish streams then run for as long as the script does:

```python
hub = mercure.Client(URL, key=KEY)                                     # every topic
reader = mercure.Client(URL, key=KEY, subscribe=["https://example.com/inbox/alice"])
```

A token that comes from elsewhere (an OAuth server, your application) is given with `token=`. The client
presents it as it is, so replace it with `set_token()` before it expires:

```python
hub = mercure.Client(URL, token=issued.access_token)   # issued: a scriptling.oauth Token
...
issued.refresh()                 # before it expires
hub.set_token(issued.access_token)
```

## Running a hub

The examples read `MERCURE_URL` (default `http://localhost:3000/.well-known/mercure`) and `MERCURE_JWT_KEY`
(default `!ChangeThisMercureHubJWTSecretKey!`, the key the hubs below are started with).

They speak Mercure 1.0, as `scriptling.mercure` does by default: tokens issued by `https://localhost` for the
hub's URL, which both hubs below trust by default, and subscriptions by exact topic or URL pattern. A hub that
speaks only draft-07 (older releases, FrankenPHP in compatibility mode, gossiper with `--protocol both`) needs
`draft07=True` beside `key=` and topic selectors or URI templates:
`hub.subscribe(topics="https://example.com/books/{id}")`.

### The official Mercure hub

```bash
docker run --rm -p 3000:80 \
  -e SERVER_NAME=':80' \
  -e MERCURE_PUBLISHER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
  -e MERCURE_SUBSCRIBER_JWT_KEY='!ChangeThisMercureHubJWTSecretKey!' \
  dunglas/mercure
```

### Gossiper

[Gossiper](https://github.com/paularlott/gossiper) is a clustered Mercure hub that also takes a streaming
publish, which `stream=True` uses:

```bash
gossiper serve --listen :3000 --http-only --publisher-key '!ChangeThisMercureHubJWTSecretKey!'
```

## Running the examples

```bash
scriptling examples/mercure/publish_subscribe.py
scriptling examples/mercure/stream_publish.py
```

Listen in one terminal and publish from another:

```bash
scriptling examples/mercure/listener.py
```

```bash
scriptling examples/mercure/notify.py "Hello there"
```
