# Test pathlib Path properties
import pathlib

# Test basic path
p = pathlib.Path("/home/user/file.txt")

assert p["name"] == "file.txt"
assert p["stem"] == "file"
assert p["suffix"] == ".txt"
assert p["parent"] == "/home/user"
assert len(p["parts"]) == 4
assert p["parts"][0] == "/"
assert p["parts"][1] == "home"
assert p["parts"][2] == "user"
assert p["parts"][3] == "file.txt"
assert p["__str__"] == "/home/user/file.txt"

# Test path without extension
p2 = pathlib.Path("/home/user/README")
assert p2["name"] == "README"
assert p2["stem"] == "README"
assert p2["suffix"] == ""

# Test root path
p3 = pathlib.Path("/")
assert p3["name"] == "/"
assert p3["stem"] == ""
assert p3["suffix"] == ""
assert p3["parent"] == "/"

# Test relative path
p4 = pathlib.Path("relative/path/file.py")
assert p4["name"] == "file.py"
assert p4["stem"] == "file"
assert p4["suffix"] == ".py"
assert p4["parent"] == "relative/path"

passed = True
passed