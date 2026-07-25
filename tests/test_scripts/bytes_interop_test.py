# End-to-end binary interop: bytes must survive every file/socket surface.
# Catches regressions where any layer forces binary through a UTF-8 string.

import os
import pathlib
import msgpack
import tempfile

# msgpack payload with high-bit-set bytes that would corrupt under UTF-8.
original = {"user": "alice", "scores": [255, 128, 0, 1, 254], "id": 99}
payload = msgpack.packb(original)
assert type(payload) == "BYTES"
assert 0xFF in payload and 0x00 in payload

# ── pathlib (the Python-canonical API for binary file I/O) ─────────────────
d = tempfile.mkdtemp()
path = d + os.sep + "data.msgpack"
p = pathlib.Path(path)

# write_bytes accepts Bytes, read_bytes returns Bytes — both correct
p.write_bytes(payload)
read_back = p.read_bytes()
assert type(read_back) == "BYTES", f"pathlib read_bytes type: {type(read_back)}"
assert read_back == payload, "pathlib round-trip corrupted bytes"
assert msgpack.unpackb(read_back) == original

# write_text / read_text still use strings
p.write_text("hello")
assert p.read_text() == "hello"
assert type(p.read_text()) == "STRING"

# ── os module convenience (write_file/append_file accept Bytes too) ────────
os.write_file(path, payload)
assert p.read_bytes() == payload

# append_file concatenates bytes cleanly
os.write_file(path, payload)
os.append_file(path, payload)
assert p.read_bytes() == payload + payload

# write_file with a String still works (backward compat)
os.write_file(path, "hello")
assert p.read_bytes() == bytes("hello")

# ── unhappy paths ──────────────────────────────────────────────────────────
# Reading a non-existent file errors cleanly (no panic)
try:
    p.read_bytes()
    assert False, "expected error on missing file (already wrote it)"
except Exception:
    pass

# pathlib.read_bytes on a missing file
missing = pathlib.Path(d + os.sep + "nope.bin")
try:
    missing.read_bytes()
    assert False, "expected error"
except Exception:
    pass

# write_bytes rejects non-string non-bytes (e.g. int)
try:
    p.write_bytes(12345)
    assert False, "expected TypeError"
except Exception:
    pass
