import glob
import tempfile
import os
import os.path
import shutil

d = tempfile.mkdtemp()
os.makedirs(d + "/sub")
os.makedirs(d + "/.hidden_dir")
os.write_file(d + "/a.txt", "x")
os.write_file(d + "/.secret.txt", "x")
os.write_file(d + "/sub/b.txt", "x")
os.write_file(d + "/.hidden_dir/c.txt", "x")

# Non-recursive: ** collapses to *
m = glob.glob("**/*.txt", d, recursive=False)
assert len(m) == 1, f"non-recursive: expected 1, got {len(m)}: {m}"

# Recursive, no hidden
m = glob.glob("**/*.txt", d, recursive=True)
assert len(m) == 2, f"recursive no hidden: expected 2, got {len(m)}: {m}"

# Recursive, include hidden
m = glob.glob("**/*.txt", d, recursive=True, include_hidden=True)
assert len(m) == 4, f"recursive hidden: expected 4, got {len(m)}: {m}"

# iglob
count = 0
for f in glob.iglob("**/*.txt", d, recursive=True):
    count += 1
assert count == 2, f"iglob: expected 2, got {count}"

# escape
assert glob.escape("file*.txt") == "file[*].txt"

shutil.rmtree(d)
True
