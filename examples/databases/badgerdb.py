#!/usr/bin/env scriptling
"""BadgerDB example: the key-value API shared with scriptling.valkey.

This is valkey.py with the connect line swapped for badger.open(): the two
plugins expose the identical surface, so a script can move between a shared
cache and local storage unchanged. Badger is embedded: one process holds the
database open at a time. Hashes are implemented natively as one key holding
the fields, so the expiry covers the whole hash.
"""

import scriptling.badgerdb as badger

print("=== BadgerDB ===\n")

client = badger.open("/tmp/scriptling-example-badger")
client.ping()

client.set("greeting", "hello from badger")
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

# batches: one transaction for many keys
client.mset({"a:1": "x", "a:2": "y"}, ttl_seconds=120)
print("mget:", client.mget("a:1", "a:2", "nope"))

# set_if_absent: take-once, atomic in a single transaction
print("set_if_absent:", client.set_if_absent("lock:report", "worker-1"),
      client.set_if_absent("lock:report", "worker-2"))

# hashes: field-value pairs under one key, expiry covers the whole hash
print("hash_set:", client.hash_set("profile:1", "user", "ada"),
      client.hash_set("profile:1", "role", "admin"))
print("hash:", client.hash_get("profile:1", "user"), client.hash_all("profile:1"))
print("hash_size/delete:", client.hash_size("profile:1"), client.hash_delete("profile:1", "role"))

print("delete:", client.delete("user:1", "user:2", "nope"))
client.delete("a:1", "a:2", "lock:report", "profile:1")

client.close()
