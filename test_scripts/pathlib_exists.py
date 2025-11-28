# Test pathlib Path exists, is_file, is_dir
import pathlib

# Create a test file
test_file = "/tmp/pathlib_test_file.txt"
p_file = pathlib.Path(test_file)
p_file.write_text("test content")

# Create a test dir
test_dir = "/tmp/pathlib_test_dir"
p_dir = pathlib.Path(test_dir)
p_dir.mkdir()

# Test exists
assert p_file.exists() == True
assert p_dir.exists() == True

p_nonexist = pathlib.Path("/tmp/nonexistent_file.txt")
assert p_nonexist.exists() == False

# Test is_file
assert p_file.is_file() == True
assert p_dir.is_file() == False
assert p_nonexist.is_file() == False

# Test is_dir
assert p_file.is_dir() == False
assert p_dir.is_dir() == True
assert p_nonexist.is_dir() == False

# Cleanup
p_file.unlink()
p_dir.rmdir()

passed = True
passed