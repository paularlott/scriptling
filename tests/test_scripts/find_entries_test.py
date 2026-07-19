import scriptling.find as find
import tempfile
import os
import os.path
import shutil

d = tempfile.mkdtemp()
os.makedirs(d + "/sub/deep")
os.makedirs(d + "/.git")
os.write_file(d + "/a.py", "x")
os.write_file(d + "/b.txt", "xxxxx")          # 5 bytes
os.write_file(d + "/big.bin", "y" * 2048)     # 2 KiB
os.write_file(d + "/sub/c.py", "x")
os.write_file(d + "/sub/deep/d.py", "x")
os.write_file(d + "/.git/config", "x")

# --- Basic shape: every entry has all four keys, types are right.
entries = find.entries(d, name="*.py", type="file")
assert len(entries) == 3, f"expected 3 .py entries, got {len(entries)}: {entries}"

sample = entries[0]
for key in ["path", "size", "mtime", "is_dir"]:
    assert key in sample, f"entry missing key {key!r}: {sample}"
assert type(sample["path"]) == type(""), f"path not str: {type(sample['path'])}"
assert type(sample["size"]) == type(0), f"size not int: {type(sample['size'])}"
assert type(sample["mtime"]) == type(0.0), f"mtime not float: {type(sample['mtime'])}"
assert type(sample["is_dir"]) == type(True), f"is_dir not bool: {type(sample['is_dir'])}"
assert sample["is_dir"] is False, f"is_dir should be False for .py file: {sample}"

# --- size is the byte length of the file.
big = find.entries(d, name="big.bin", type="file")
assert len(big) == 1, f"expected 1 big.bin, got {len(big)}"
assert big[0]["size"] == 2048, f"size: got {big[0]['size']}, want 2048"
assert big[0]["path"].endswith("/big.bin"), f"path: {big[0]['path']}"

# --- is_dir flag for directory matches.
dirs = find.entries(d, type="dir")
assert len(dirs) == 2, f"expected 2 dirs (no hidden), got {len(dirs)}"
for e in dirs:
    assert e["is_dir"] is True, f"dir entry with is_dir=False: {e}"

# --- mtime is epoch seconds (float) and matches os.path.getmtime.
import time
allowance = 5  # seconds tolerance for fs timestamp rounding
fs_mtime = os.path.getmtime(big[0]["path"])
assert abs(big[0]["mtime"] - fs_mtime) <= allowance, (
    f"mtime drift: entries={big[0]['mtime']}, getmtime={fs_mtime}")

# --- entries() and path() agree on the matching set.
plain = find.path(d, type="any")
rich = find.entries(d, type="any")
assert len(plain) == len(rich), (
    f"path/entries disagree on count: path={len(plain)} entries={len(rich)}")

plain_set = {p for p in plain}
rich_set = {e["path"] for e in rich}
assert plain_set == rich_set, (
    f"path/entries disagree on set:\n path={plain_set}\n entries={rich_set}")

# --- Doc example 1: build a {path: mtime} index via dict comprehension.
mtimes = {e["path"]: e["mtime"] for e in find.entries(d, type="file")}
assert len(mtimes) == 5, f"expected 5 files in index, got {len(mtimes)}"
# Every key is a path that exists in plain path() output.
for p in mtimes.keys():
    assert p in plain_set, f"index key not in path() set: {p}"

# --- Doc example 2: iterate entries and read size in the loop body.
total = 0
for e in find.entries(d, name="*.bin", type="file"):
    total += e["size"]
assert total == 2048, f"expected total size 2048, got {total}"

# --- Filters carry over from path() unchanged.
large = find.entries(d, type="file", size_min=1000)
assert len(large) == 1, f"size_min filter: expected 1, got {len(large)}"
assert large[0]["path"].endswith("/big.bin")

top_hidden = find.entries(d, type="dir", include_hidden=True)
assert len(top_hidden) == 3, f"include_hidden: expected 3 dirs, got {len(top_hidden)}"

shutil.rmtree(d)
True
