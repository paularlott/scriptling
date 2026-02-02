# Comprehensive test os and os.path libraries
import os
import os.path
import platform

passed = True

# Test 1: Environment variables
print("Testing environment variables...")
home = os.getenv("HOME", "/default")
if home == "":
    home = "/default"
passed = passed and len(home) > 0

# Test getenv with default
test_env = os.getenv("NONEXISTENT_VAR", "default_value")
assert test_env == "default_value"

# Test environ dict
environ = os.environ()
assert isinstance(environ, "dict")
assert "HOME" in environ or "USERPROFILE" in environ  # Cross-platform

# Test 2: Current working directory
print("Testing current working directory...")
cwd = os.getcwd()
passed = passed and len(cwd) > 0
assert cwd.startswith("/")

# Test 3: OS constants
print("Testing OS constants...")
assert os.name in ["posix", "nt"]
assert os.sep in ["/", "\\"]
assert os.linesep in ["\n", "\r\n"]

# Test 4: Directory listing
print("Testing directory listing...")
entries = os.listdir("/tmp")  # Use /tmp which should exist
assert isinstance(entries, "list")
assert len(entries) >= 0  # At least empty list

# Test 5: File operations
print("Testing file operations...")
test_file = "/tmp/os_test_file.txt"
test_content = "Hello, OS test!"

# Write file
os.write_file(test_file, test_content)

# Read file
read_content = os.read_file(test_file)
assert read_content == test_content

# Append file
os.append_file(test_file, "\nAppended content")
appended_content = os.read_file(test_file)
assert appended_content == test_content + "\nAppended content"

# Test 6: Directory operations
print("Testing directory operations...")
test_dir = "/tmp/os_test_dir"
nested_dir = "/tmp/os_test_dir/nested"

# Create directory
os.mkdir(test_dir)
assert os.path.exists(test_dir)
assert os.path.isdir(test_dir)

# Create nested directories
os.makedirs(nested_dir)
assert os.path.exists(nested_dir)
assert os.path.isdir(nested_dir)

# Test 7: File/directory checks
print("Testing file/directory checks...")
assert os.path.exists(test_file)
assert os.path.isfile(test_file)
assert not os.path.isdir(test_file)

assert os.path.exists(test_dir)
assert os.path.isdir(test_dir)
assert not os.path.isfile(test_dir)

assert not os.path.exists("/tmp/nonexistent_file_12345")

# Test 8: Path operations
print("Testing path operations...")
path = "/usr/local/bin/python3"
assert os.path.dirname(path) == "/usr/local/bin"
assert os.path.basename(path) == "python3"

parts = os.path.split(path)
assert parts[0] == "/usr/local/bin"
assert parts[1] == "python3"

result = os.path.splitext("/home/user/file.txt")
assert result[0] == "/home/user/file"
assert result[1] == ".txt"

# Test absolute path
abs_path = os.path.abspath("relative/path")
assert os.path.isabs(abs_path)

# Test relative path
rel_path = os.path.relpath("/home/user/docs", "/home")
assert rel_path == "user/docs"

# Test path normalization
norm_path = os.path.normpath("/home/user/../other/file.txt")
assert norm_path == "/home/other/file.txt"

# Test 9: File size
print("Testing file size...")
size = os.path.getsize(test_file)
assert size > 0  # Should have content

# Test 10: Rename/move
print("Testing rename...")
renamed_file = "/tmp/os_test_file_renamed.txt"
os.rename(test_file, renamed_file)
assert not os.path.exists(test_file)
assert os.path.exists(renamed_file)

# Test 11: Remove operations
print("Testing remove operations...")
os.remove(renamed_file)
assert not os.path.exists(renamed_file)

# Remove directories
os.rmdir(nested_dir)
assert not os.path.exists(nested_dir)

os.rmdir(test_dir)
assert not os.path.exists(test_dir)

# Test 12: Platform information
print("Testing platform information...")
system = platform.system()
assert system in ["Darwin", "Linux", "Windows", "FreeBSD"]
machine = platform.machine()
assert len(machine) > 0
version = platform.scriptling_version()
assert len(version) > 0

# Test 13: Path joining
print("Testing path joining...")
joined = os.path.join("/home", "user", "docs")
assert joined == "/home/user/docs" or joined == "/home\\user\\docs"

# Test 14: Edge cases
print("Testing edge cases...")
# Empty dirname/basename (Go behavior, not Python)
assert os.path.dirname("") == "."
assert os.path.basename("") == "."

# Root paths
assert os.path.dirname("/") == "/"
assert os.path.basename("/") == "/"

# Extensions
assert os.path.splitext("file") == ("file", "")
assert os.path.splitext("file.txt") == ("file", ".txt")
assert os.path.splitext(".hidden") == ("", ".hidden")

print("All comprehensive OS tests passed!")
assert passed
