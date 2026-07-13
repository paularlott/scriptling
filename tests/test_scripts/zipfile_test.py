import zipfile
import tempfile
import os
import os.path
import shutil

d = tempfile.mkdtemp()
zpath = d + "/test.zip"

# Create
zf = zipfile.ZipFile(zpath, "w")
zf.writestr("hello.txt", "hello world")
zf.writestr("sub/deep.txt", "deep content")
zf.close()

assert zipfile.is_zipfile(zpath), "should be valid zip"

# Read
zf = zipfile.ZipFile(zpath)
names = zf.namelist()
assert len(names) == 2, f"expected 2 entries, got {len(names)}"
assert "hello.txt" in names, f"missing hello.txt: {names}"
content = zf.read("hello.txt")
assert content == "hello world", f"content mismatch: {content}"

# Extract one
extracted = zf.extract("hello.txt", d + "/out")
assert os.path.exists(extracted), f"extract should create file: {extracted}"

# Extract all
paths = zf.extractall(d + "/all")
assert len(paths) == 2, f"extractall should return 2 paths"
zf.close()

shutil.rmtree(d)
True
