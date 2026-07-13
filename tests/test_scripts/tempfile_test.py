import tempfile
import os
import os.path
import shutil

# mkstemp creates a file
f = tempfile.mkstemp(prefix="test_")
assert os.path.exists(f), "mkstemp should create a file"
os.remove(f)

# mkdtemp creates a directory
d = tempfile.mkdtemp(prefix="dir_")
assert os.path.isdir(d), "mkdtemp should create a directory"

# suffix and prefix
f = tempfile.mkstemp(prefix="data_", suffix=".json")
assert "data_" in os.path.basename(f), f"prefix not in name: {f}"
assert f.endswith(".json"), f"suffix not in name: {f}"
os.remove(f)

# dir= parameter
f = tempfile.mkstemp(dir=d)
assert os.path.dirname(f) == os.path.normpath(d), f"dir mismatch: {f}"
os.remove(f)

# gettempdir
td = tempfile.gettempdir()
assert len(td) > 0, "gettempdir should be non-empty"

# gettempprefix
assert tempfile.gettempprefix() == "tmp", "prefix should be 'tmp'"

shutil.rmtree(d)
True
