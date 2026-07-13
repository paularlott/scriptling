import scriptling.find as find
import tempfile
import os
import os.path
import shutil

d = tempfile.mkdtemp()
os.makedirs(d + "/sub/deep")
os.makedirs(d + "/.git")
os.write_file(d + "/a.py", "x")
os.write_file(d + "/b.txt", "x")
os.write_file(d + "/sub/c.py", "x")
os.write_file(d + "/sub/deep/d.py", "x")
os.write_file(d + "/.git/config", "x")

# Find all .py files
py = find.path(d, name="*.py", type="file")
assert len(py) == 3, f"expected 3 .py files, got {len(py)}: {py}"

# Find dirs (no hidden)
dirs = find.path(d, type="dir")
assert len(dirs) == 2, f"expected 2 dirs, got {len(dirs)}: {dirs}"

# Find dirs with hidden
dirs = find.path(d, type="dir", include_hidden=True)
assert len(dirs) == 3, f"expected 3 dirs, got {len(dirs)}: {dirs}"

# max_depth=1 (immediate children only)
top = find.path(d, recursive=True, max_depth=1)
names = sorted([os.path.basename(p) for p in top])
assert names == ["a.py", "b.txt", "sub"], f"max_depth=1: {names}"

# Non-recursive (same as max_depth=1)
top2 = find.path(d, recursive=False)
assert len(top2) == len(top), f"non-recursive mismatch"

shutil.rmtree(d)
True
