# Tests for os.read_lines() — lazy line-by-line file iteration.
# Covers: basic iteration, empty file, CRLF handling, early break,
# comprehension, comparison with read_file().splitlines(), error cases.

import os
import tempfile

d = tempfile.mkdtemp()

# ── Happy path: multi-line file ────────────────────────────────────────────
path = d + os.sep + "lines.txt"
os.write_file(path, "alpha\nbeta\ngamma\ndelta\n")

lines = []
for line in os.read_lines(path):
    lines.append(line)
assert lines == ["alpha", "beta", "gamma", "delta"], f"got {lines}"

# ── No trailing newline on last line ───────────────────────────────────────
path2 = d + os.sep + "notrail.txt"
os.write_file(path2, "one\ntwo\nthree")
lines = []
for line in os.read_lines(path2):
    lines.append(line)
assert lines == ["one", "two", "three"], f"got {lines}"

# ── Empty file → zero iterations ───────────────────────────────────────────
path3 = d + os.sep + "empty.txt"
os.write_file(path3, "")
count = 0
for _ in os.read_lines(path3):
    count += 1
assert count == 0

# ── Single line, no newline ────────────────────────────────────────────────
path4 = d + os.sep + "single.txt"
os.write_file(path4, "only")
lines = []
for line in os.read_lines(path4):
    lines.append(line)
assert lines == ["only"]

# ── CRLF line endings are stripped ─────────────────────────────────────────
path5 = d + os.sep + "crlf.txt"
os.write_file(path5, bytes([0x61, 0x0D, 0x0A, 0x62, 0x0D, 0x0A, 0x63]))  # a\r\nb\r\nc
lines = []
for line in os.read_lines(path5):
    lines.append(line)
assert lines == ["a", "b", "c"], f"got {lines}"

# ── Early break (file handle should be cleaned up by GC) ───────────────────
path6 = d + os.sep + "big.txt"
content = ""
for i in range(1000):
    content += f"line {i}\n"
os.write_file(path6, content)

collected = []
for line in os.read_lines(path6):
    collected.append(line)
    if len(collected) == 5:
        break
assert len(collected) == 5
assert collected[0] == "line 0"
assert collected[4] == "line 4"

# ── Comprehension works (iterator is a real iterable) ──────────────────────
uppered = [line.upper() for line in os.read_lines(path)]
assert uppered == ["ALPHA", "BETA", "GAMMA", "DELTA"]

# ── Consistency with read_file().splitlines() ─────────────────────────────
# For small files, both produce the same result.
via_read_lines = list(os.read_lines(path))
via_splitlines = os.read_file(path).splitlines()
assert via_read_lines == via_splitlines, "read_lines and splitlines disagree"

# ── Unhappy paths ──────────────────────────────────────────────────────────
# Missing file
try:
    for _ in os.read_lines(d + os.sep + "nonexistent"):
        pass
    assert False, "expected error for missing file"
except Exception:
    pass

# Non-string argument
try:
    os.read_lines(12345)
    assert False, "expected TypeError"
except Exception:
    pass

# No arguments
try:
    os.read_lines()
    assert False, "expected error for no args"
except Exception:
    pass
