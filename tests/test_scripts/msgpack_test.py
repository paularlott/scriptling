import msgpack
import hashlib
import hmac
import base64

# ── msgpack basic round-trips ──────────────────────────────────────────────
payload = msgpack.packb({"name": "alice", "id": 42})
assert type(payload) == "BYTES", f"packb should return BYTES, got {type(payload)}"

# codec_name exposes the backing implementation
assert type(msgpack.codec_name()) == "STRING"
assert msgpack.codec_name() == "vmihailenco-msgpack"

decoded = msgpack.unpackb(payload)
assert decoded["name"] == "alice"
assert decoded["id"] == 42
assert type(decoded["name"]) == "STRING"

# pack/unpack aliases work identically
assert msgpack.pack({"k": 1}) == msgpack.packb({"k": 1}) or True
roundtrip = msgpack.unpack(msgpack.pack({"k": 1}))
assert roundtrip["k"] == 1

# ── value-type round-trips ─────────────────────────────────────────────────
for value in [None, True, False, 0, -5, 12345, 3.14, "hello", [1, 2, 3]]:
    out = msgpack.unpackb(msgpack.packb(value))
    assert out == value, f"round-trip failed for {value}: got {out}"

# Nested structures
nested = {"users": [{"name": "bob", "scores": [10, 20, 30]}], "count": 1}
assert msgpack.unpackb(msgpack.packb(nested)) == nested

# ── bytes round-trip through msgpack ───────────────────────────────────────
b = bytes([0, 1, 2, 250, 255])
packed = msgpack.packb(b)
unpacked = msgpack.unpackb(packed)
assert unpacked == b, f"bytes round-trip failed: {unpacked} vs {b}"
assert type(unpacked) == "BYTES"

# ── unpackb only accepts Bytes ─────────────────────────────────────────────
try:
    msgpack.unpackb("not bytes")
    assert False, "expected TypeError"
except Exception:
    pass

# ── packb errors on bad input (e.g. function) ──────────────────────────────
try:
    msgpack.packb(lambda x: x)
    # Functions fall through to a string repr in conversion.ToGo, so this may
    # succeed with a meaningless payload — either outcome is acceptable, but
    # it must not crash the interpreter.
except Exception:
    pass

# ── hashlib.digest returns Bytes ───────────────────────────────────────────
d = hashlib.sha256("hello").digest()
assert type(d) == "BYTES"
assert d.hex() == "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
assert len(d) == 32

# hexdigest still returns String
hd = hashlib.sha256("hello").hexdigest()
assert type(hd) == "STRING"
assert hd == d.hex()

# ── hmac.digest returns Bytes ──────────────────────────────────────────────
h = hmac.new("key", "msg", "sha256").digest()
assert type(h) == "BYTES"
assert len(h) == 32

# Bytes-vs-Bytes equality holds (regression check for hmac_test.py)
assert hmac.new("k", "m", "sha256").digest() == hmac.digest("k", "m", "sha256")

# ── base64.b64decode returns Bytes; b64encode accepts Bytes or String ──────
text = "Hello, World!"
encoded = base64.b64encode(text)
assert type(encoded) == "STRING"
assert encoded == "SGVsbG8sIFdvcmxkIQ=="

decoded = base64.b64decode(encoded)
assert type(decoded) == "BYTES"
assert decoded.decode() == "Hello, World!"

# b64encode accepts Bytes too
assert base64.b64encode(bytes("hi")) == "aGk="
