import base64

# ── bytes constructor: empty ───────────────────────────────────────────────
b = bytes()
assert len(b) == 0
assert b == bytes("")

# ── bytes constructor: from string ─────────────────────────────────────────
b = bytes("hi")
assert type(b) == "BYTES"
assert len(b) == 2
assert b.decode() == "hi"

# ── bytes constructor: from list/tuple of ints ─────────────────────────────
b = bytes([104, 105])
assert b == bytes("hi")
b = bytes((104, 105))
assert b == bytes("hi")

# Out-of-range int rejected
try:
    bytes([256])
    assert False, "expected error"
except Exception:
    pass
try:
    bytes([-1])
    assert False, "expected error"
except Exception:
    pass

# Non-int element rejected
try:
    bytes(["x"])
    assert False, "expected error"
except Exception:
    pass

# ── bytes constructor: only utf-8 encoding accepted ────────────────────────
try:
    bytes("hi", "ascii")
    assert False, "expected error"
except Exception:
    pass

# ── bytes.fromhex / bytes.frombase64 ───────────────────────────────────────
assert bytes.fromhex("6869") == bytes("hi")
assert bytes.frombase64("aGk=") == bytes("hi")

# ── Bytes methods ──────────────────────────────────────────────────────────
b = bytes([0x68, 0x69, 0x30, 0x39])
assert b.decode() == "hi09"
assert b.hex() == "68693039"
assert b.base64() == "aGkwOQ=="
assert b.length() == 4
assert len(b) == 4

# decode with unsupported encoding rejected
try:
    b.decode("ascii")
    assert False, "expected error"
except Exception:
    pass

# ── indexing & slicing ─────────────────────────────────────────────────────
b = bytes("hello")
assert b[0] == 104
assert b[-1] == 111
assert b[1:3] == bytes("el")
assert b[:2] == bytes("he")
assert b[-2:] == bytes("lo")
assert b[::2] == bytes("hlo")
assert b[::-1] == bytes("olleh")

# Indexing out of range returns None (matches String)
assert b[100] is None

# ── iteration yields ints ──────────────────────────────────────────────────
collected = []
for v in bytes("ab"):
    collected.append(v)
assert collected == [97, 98]

# Comprehension-style iteration works
assert [x for x in bytes("ab")] == [97, 98]
assert sum(bytes([1, 2, 3])) == 6

# ── concatenation & repetition ─────────────────────────────────────────────
assert bytes("ab") + bytes("cd") == bytes("abcd")
assert bytes("ab") * 3 == bytes("ababab")
assert 3 * bytes("ab") == bytes("ababab")
assert bytes("x") * 0 == bytes()
assert bytes("x") * -1 == bytes()

# ── equality / ordering against other Bytes ────────────────────────────────
assert bytes("abc") == bytes("abc")
assert bytes("abc") != bytes("abd")
assert bytes("abc") < bytes("abd")
assert bytes("abc") <= bytes("abc")
assert bytes("abd") > bytes("abc")
assert bytes("abc") >= bytes("abc")

# ── in operator (int in bytes / bytes in bytes) ────────────────────────────
b = bytes("hello")
assert 104 in b      # 'h'
assert 122 not in b  # 'z'
assert bytes("ell") in b
assert bytes("xyz") not in b

# Non-int/Bytes needles are type errors
try:
    "h" in b
    assert False, "expected TypeError"
except Exception:
    pass

# ── strict mixing with String raises ───────────────────────────────────────
try:
    "x" + bytes("y")
    assert False, "expected TypeError"
except Exception:
    pass
try:
    bytes("x") + "y"
    assert False, "expected TypeError"
except Exception:
    pass

# ── Bytes are truthy when non-empty ────────────────────────────────────────
assert bytes("x")
assert not bytes()

# ── Inspect / repr format ──────────────────────────────────────────────────
assert bytes("hi").hex() == "6869"  # sanity

# ── hashlib/hmac round-trip via Bytes ──────────────────────────────────────
import hashlib
d = hashlib.sha256("hello").digest()
assert type(d) == "BYTES"
# Verify the digest can be re-encoded as base64 / hex without corruption
import base64
assert len(base64.b64encode(d)) > 0
assert len(d.hex()) == 64

# ── bytes survives a msgpack round-trip with binary content ────────────────
import msgpack
binary = bytes([0, 128, 255, 1, 254])
assert msgpack.unpackb(msgpack.packb(binary)) == binary
