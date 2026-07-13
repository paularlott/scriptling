import tarfile
import tempfile
import os
import os.path
import shutil

d = tempfile.mkdtemp()
tpath = d + "/test.tar.gz"

# Create gzipped
tf = tarfile.TarFile(tpath, "w:gz")
tf.addstr("hello.txt", "hello tar")
tf.addstr("sub/deep.txt", "deep tar")
tf.close()

assert tarfile.is_tarfile(tpath), "should be valid tar"

# Read gzipped
tf = tarfile.TarFile(tpath, "r:gz")
names = tf.getnames()
assert len(names) == 2, f"expected 2 entries, got {len(names)}"
content = tf.read("hello.txt")
assert content == "hello tar", f"content mismatch: {content}"

# Extract all
paths = tf.extractall(d + "/out")
assert len(paths) == 2, f"extractall: expected 2 paths"
tf.close()

# Read plain (uncompressed) tar
plain = d + "/plain.tar"
tf = tarfile.TarFile(plain, "w")
tf.addstr("a.txt", "aaa")
tf.close()

tf = tarfile.TarFile(plain, "r")
assert tf.read("a.txt") == "aaa", "plain tar read"
tf.close()

shutil.rmtree(d)
True
