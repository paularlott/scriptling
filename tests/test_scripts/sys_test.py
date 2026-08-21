import sys

# Test sys.argv
argv = sys.argv
assert isinstance(argv, "list")

# Test sys.platform
platform = sys.platform
assert platform in ["darwin", "linux", "win32"]

# Test sys.version
version = sys.version
assert len(version) > 0

# Test sys.path_sep
path_sep = sys.path_sep
assert len(path_sep) == 1

# Test sys.maxsize
maxsize = sys.maxsize
assert maxsize == 9223372036854775807

# Test sys.stdout / sys.stderr stream objects.
# Empty writes/prints keep the test output clean.
assert sys.stdout.write("") == 0
assert sys.stderr.write("") == 0
assert sys.stdout.writelines([]) is None
assert sys.stdout.flush() is None
assert sys.stderr.isatty() in [True, False]
assert print("", file=sys.stderr, end="") is None
with sys.stdout as f:
    assert f.write("") == 0