# Test pathlib Path file operations
import pathlib

# Test write_text and read_text
test_file = "/tmp/pathlib_test_rw.txt"
p = pathlib.Path(test_file)

content = "Hello, World!\nThis is a test."
p.write_text(content)

read_content = p.read_text()
assert read_content == content

# Test unlink
assert p.exists() == True
p.unlink()
assert p.exists() == False

# Test unlink missing_ok
p.unlink(missing_ok=True)  # Should not error

passed = True
passed