# Tests os.symlink and os.path.islink through the interpreter.
import os
import os.path
import tempfile
import shutil

d = tempfile.mkdtemp()
target = d + "/target.txt"
link = d + "/link.txt"
missing = d + "/nonexistent"

os.write_file(target, "hello world")

# Symlink support is platform-dependent (Windows requires privileges).
try:
    os.symlink(target, link)
    symlink_supported = True
except Exception:
    symlink_supported = False

if symlink_supported:
    # islink True for the symlink itself.
    assert os.path.islink(link) is True, "islink(link) should be True"

    # islink False for a regular file (it's a file, not a link).
    assert os.path.islink(target) is False, "islink(target) should be False for regular file"

    # islink False for a missing path (Lstat fails, returns False not an error).
    assert os.path.islink(missing) is False, "islink(missing) should be False"

    # Reading through the symlink resolves to the target's content.
    assert os.read_file(link) == "hello world", "symlink should resolve for read_file"

    # exists/isfile follow the link to the target.
    assert os.path.exists(link), "symlink should exist"
    assert os.path.isfile(link), "symlink to file should be isfile (follows link)"

    # Relative target as well.
    rel_link = d + "/rel_link.txt"
    os.symlink("target.txt", rel_link)
    assert os.path.islink(rel_link) is True, "islink(rel_link) should be True"
    assert os.read_file(rel_link) == "hello world", "relative symlink should resolve"
else:
    # Without symlink support, islink returns False without raising.
    assert os.path.islink(target) is False
    assert os.path.islink(missing) is False

shutil.rmtree(d)
True
