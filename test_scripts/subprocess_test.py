import subprocess

# Test basic command execution
result = subprocess.run("echo hello")
assert result.returncode == 0

# Test with capture_output
result = subprocess.run("echo hello world", {"capture_output": True})
assert result.returncode == 0
assert result.stdout.strip() == "hello world"

# Test with list args
result = subprocess.run(["echo", "test"])
assert result.returncode == 0

# Test check option with successful command
result = subprocess.run("true", {"check": True})
assert result.returncode == 0

# Test command that fails
result = subprocess.run("false")
assert result.returncode == 1

# Test check option with failing command (should raise exception)
try:
    subprocess.run("false", {"check": True})
    assert False, "Should have raised exception"
except:
    pass  # Expected

# Test shell option
result = subprocess.run("echo 'shell test'", {"shell": True, "capture_output": True})
assert result.returncode == 0
assert result.stdout.strip() == "shell test"

True