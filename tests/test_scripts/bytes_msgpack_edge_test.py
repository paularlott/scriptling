# Edge-case coverage for bytes and msgpack that isn't covered by the
# basic round-trip scripts. Each section is a behaviour we want to lock in.

import msgpack
import hashlib
import hmac
import base64

# ── Bytes is hashable: usable as dict keys and set elements ────────────────
# Duplicate keys (by content equality) collapse: last value wins, like Python.
d = {bytes("a"): 1, bytes("b"): 2, bytes("a"): 3}
assert len(d) == 2
assert d[bytes("a")] == 3
assert d[bytes("b")] == 2

# Different bytes values with same contents are the same key
d2 = {}
d2[bytes("k")] = "first"
d2[bytes("k")] = "second"   # overwrites — proves same key
assert len(d2) == 1
assert d2[bytes("k")] == "second"

# Set membership uses content equality, not pointer identity
s = {bytes("a"), bytes("b"), bytes("a"), bytes("a")}
assert len(s) == 2
assert bytes("a") in s
assert bytes("z") not in s

# Bytes inside tuples remain hashable
td = {(bytes("k"), "tag"): 99}
assert td[(bytes("k"), "tag")] == 99

# ── Iterable builtins consume bytes as integer values ──────────────────────
b = bytes([5, 3, 8, 1, 9, 3])
assert list(b) == [5, 3, 8, 1, 9, 3]
assert tuple(b) == (5, 3, 8, 1, 9, 3)
assert min(b) == 1
assert max(b) == 9
assert sum(b) == 29
assert sorted(b) == [1, 3, 3, 5, 8, 9]
# any/all check truthiness of integer byte values (0 is falsy)
assert any(bytes([0, 0, 1])) is True
assert all(bytes([1, 2, 0])) is False

# ── Bytes inside containers: deep equality and membership ──────────────────
assert [bytes("a"), bytes("b")] == [bytes("a"), bytes("b")]
assert [bytes("a")] in [[bytes("a")], [bytes("x")]]
assert {"k": bytes("v")} == {"k": bytes("v")}

# ── Bytes in format contexts renders as Python-style repr ──────────────────
b = bytes("hi")
assert f"{b}" == "b'hi'"           # f-string interpolation uses Inspect
assert "value=%s" % b == "value=b'hi'"
# Non-printable bytes show \x.. escapes in the repr
assert f"{bytes([0, 0xff])}" == "b'\\x00\\xff'"

# ── Bytes constructor: keyword-form encoding ───────────────────────────────
assert bytes("hi", encoding="utf-8") == bytes("hi")
assert bytes("hi", "utf-8") == bytes("hi")

# ── Bytes methods: keyword-form encoding ───────────────────────────────────
assert bytes("hi").decode(encoding="utf-8") == "hi"

# ── Empty-bytes edge cases ─────────────────────────────────────────────────
eb = bytes()
assert len(eb) == 0
assert bool(eb) is False
assert eb[0] is None              # Scriptling convention: OOR index → None
assert eb[10:20] == bytes()       # slicing empty is empty
assert eb.hex() == ""
assert eb.base64() == ""
assert eb.decode() == ""
# All comparisons against itself hold
assert eb == bytes()
assert eb <= bytes()
assert eb >= bytes()

# ── Bytes survives every binary-producing/consuming library ────────────────
# hashlib with bytes input → bytes output → re-encodable
h = hashlib.sha256(bytes("hello")).digest()
assert type(h) == "BYTES"
assert base64.b64encode(h) == base64.b64encode(hashlib.sha256("hello").digest())
# hmac with bytes input
m = hmac.new(bytes("key"), bytes("msg"), "sha256").digest()
assert type(m) == "BYTES"
# base64 round-trip via bytes
roundtrip = base64.b64decode(base64.b64encode(bytes("payload")))
assert roundtrip == bytes("payload")

# ── msgpack edge cases ─────────────────────────────────────────────────────
assert msgpack.unpackb(msgpack.packb(None)) is None
assert msgpack.unpackb(msgpack.packb({})) == {}
assert msgpack.unpackb(msgpack.packb([])) == []
# Unicode round-trips losslessly
assert msgpack.unpackb(msgpack.packb("hélloωörld")) == "hélloωörld"
# int64 boundaries (the lexer accepts up to int64 max; min_int64 is computed
# via arithmetic to avoid the literal `-9223372036854775808` overflowing).
min_int64 = -9223372036854775807 - 1
assert msgpack.unpackb(msgpack.packb(9223372036854775807)) == 9223372036854775807
assert msgpack.unpackb(msgpack.packb(-9223372036854775807)) == -9223372036854775807
assert msgpack.unpackb(msgpack.packb(min_int64)) == min_int64
# Float precision preserved
assert msgpack.unpackb(msgpack.packb(3.14159265358979)) == 3.14159265358979
# Deeply nested structures
deep = {"a": {"b": {"c": [1, 2, {"d": 4, "e": [bytes("x"), True, None]}]}}}
assert msgpack.unpackb(msgpack.packb(deep)) == deep
# Mixed-type list including bytes
mixed = msgpack.unpackb(msgpack.packb([1, "a", None, [2, 3], True, 3.14, bytes([0, 1])]))
assert mixed[0] == 1 and mixed[1] == "a" and mixed[2] is None
assert mixed[3] == [2, 3] and mixed[4] is True
assert mixed[6] == bytes([0, 1])

# ── msgpack: truly invalid bytes raise ─────────────────────────────────────
# 0xc1 is reserved as "never-used" in the msgpack spec.
try:
    msgpack.unpackb(bytes([0xc1, 0x02]))
    assert False, "expected error on 0xc1"
except Exception:
    pass
# Truncated fixstr payload
try:
    msgpack.unpackb(bytes([0xae, 0x61, 0x62, 0x63]))  # promises 14 bytes, only 3 follow
    assert False, "expected error on truncated payload"
except Exception:
    pass
