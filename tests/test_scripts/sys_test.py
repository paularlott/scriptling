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

# Test sys.exit() raises catchable exception
try:
    sys.exit(42)
    assert False, "sys.exit should raise exception"
except Exception as e:
    # Should catch SystemExit exception
    msg = str(e)
    assert "SystemExit" in msg, "Exception should contain 'SystemExit'"
    assert "42" in msg, "Exception should contain exit code"

# Test sys.exit() with string message
try:
    sys.exit("custom error")
    assert False, "sys.exit with string should raise exception"
except Exception as e:
    assert str(e) == "custom error", "Exception message should match"