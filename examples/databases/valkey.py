#!/usr/bin/env scriptling
"""Valkey / Redis example: the key-value API shared with scriptling.badgerdb.

Covers connect, set with TTL, get, incr/decr, exists, ttl, keys, expire,
persist, batch mget/mset, set_if_absent, hashes, delete, and close. The
shared-core part runs against scriptling.badgerdb with only the connect line
changed: see badgerdb.py. Sets, queues and select() are valkey-only.
"""

import os
import scriptling.valkey as valkey

URL = os.getenv("SCRIPTLING_VALKEY_URL", "valkey://127.0.0.1:16379")

print("=== Valkey / Redis ===\n")

client = valkey.connect(URL)
client.ping()

client.set("greeting", "hello from valkey")
client.set("session:42", "expires soon", ttl_seconds=60)

print("get:", client.get("greeting"))
print("missing:", client.get("nope"))

print("incr:", client.incr("hits"), client.incr("hits", amount=10), client.decr("hits", amount=2))

print("exists:", client.exists("greeting", "session:42", "nope"))
print("ttl (no expiry):", client.ttl("greeting"))
print("ttl (60s):", client.ttl("session:42"))
print("ttl (missing):", client.ttl("nope"))

client.set("user:1", "ada")
client.set("user:2", "grace")
print("keys user:*:", client.keys("user:*"))

print("expire:", client.expire("user:1", 120))
print("persist:", client.persist("session:42"), "ttl now:", client.ttl("session:42"))

# batches: one round trip for many keys
client.mset({"a:1": "x", "a:2": "y"}, ttl_seconds=120)
print("mget:", client.mget("a:1", "a:2", "nope"))

# set_if_absent: take-once, the lock primitive
print("set_if_absent:", client.set_if_absent("lock:report", "worker-1"),
      client.set_if_absent("lock:report", "worker-2"))

# hashes: field-value pairs under one key, expiry covers the whole hash
print("hash_set:", client.hash_set("profile:1", "user", "ada"),
      client.hash_set("profile:1", "role", "admin"))
print("hash:", client.hash_get("profile:1", "user"), client.hash_all("profile:1"))
print("hash_size/delete:", client.hash_size("profile:1"), client.hash_delete("profile:1", "role"))

print("delete:", client.delete("user:1", "user:2", "nope"))
client.delete("a:1", "a:2", "lock:report", "profile:1")

# sets and queues are valkey-only (native redis types)
client.set_add("crew", "ada", "grace")
print("set:", client.set_size("crew"), client.set_contains("crew", "ada"))
client.queue_push("jobs", "resize", "email")
print("queue:", client.queue_peek("jobs"), client.queue_pop("jobs"), client.queue_size("jobs"))
client.queue_push("jobs", "queued")
print("queue_wait immediate:", client.queue_wait("jobs", 1))
client.delete("jobs")
print("queue_wait timeout (queue empty):", client.queue_wait("jobs", 0.2))

# database selection: switch once per phase, not per command
client.set("home", "db0")
client.select(2)
client.set("away", "db2")
print("db2 sees db0 keys:", client.get("home") != None)
client.select(0)
print("back home:", client.get("home"))

# mode() reports how the client talks to the server; flushdb clears the
# current database (a cluster gets it on every node that accepts writes)
print("mode:", client.mode())
client.flushdb()
print("keys after flushdb:", client.keys("*"))

# clusters and sentinels: pass seed addresses and a mode
#   valkey.connect("valkey://node-a:7000,node-b:7000", mode="cluster")
#   valkey.connect("valkey://s1:26379", mode="sentinel", master_set="mymaster")

client.close()
