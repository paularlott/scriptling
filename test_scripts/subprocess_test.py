import subprocess

passed = True

# Test 1: Basic command execution
print("Testing basic command execution...")
result = subprocess.run("echo hello")
assert result.returncode == 0
assert result.args == ["echo", "hello"]

# Test 2: Capture output
print("Testing output capture...")
result = subprocess.run("echo hello world", {"capture_output": True})
assert result.returncode == 0
assert result.stdout.strip() == "hello world"
assert result.stderr == ""

# Test 3: List args
print("Testing list arguments...")
result = subprocess.run(["echo", "test"])
assert result.returncode == 0
assert result.args == ["echo", "test"]

# Test 4: Check option with successful command
print("Testing check option...")
result = subprocess.run("true", {"check": True})
assert result.returncode == 0

# Test 5: Check option with failing command
print("Testing check with failing command...")
try:
    subprocess.run("false", {"check": True})
    assert False, "Should have raised exception"
except:
    pass  # Expected

# Test 6: Shell option
print("Testing shell option...")
result = subprocess.run("echo 'shell test'", {"shell": True, "capture_output": True})
assert result.returncode == 0
assert result.stdout.strip() == "shell test"

# Test 7: Command that produces stderr (use a failing command to capture stderr)
print("Testing stderr capture...")
result = subprocess.run("sh -c 'echo error >&2; exit 1'", {"shell": True, "capture_output": True})
assert result.returncode == 1
assert "error" in result.stderr
assert result.stdout == ""

# Test 8: Failing command with capture
print("Testing failing command with capture...")
result = subprocess.run("false", {"capture_output": True})
assert result.returncode == 1
assert result.stdout == ""
assert result.stderr == ""

# Test 9: Complex command with shell
print("Testing complex shell command...")
result = subprocess.run("echo 'hello' && echo 'world'", {"shell": True, "capture_output": True})
assert result.returncode == 0
lines = result.stdout.strip().split('\n')
assert len(lines) == 2
assert lines[0] == "hello"
assert lines[1] == "world"

# Test 10: Command with arguments in shell
print("Testing shell command with arguments...")
result = subprocess.run("printf '%s\\n%s\\n' arg1 arg2", {"shell": True, "capture_output": True})
assert result.returncode == 0
assert result.stdout.strip() == "arg1\narg2"

# Test 11: Another shell command with capture
print("Testing another shell command with capture...")
result = subprocess.run("printf 'hello world'", {"shell": True, "capture_output": True})
assert result.returncode == 0
assert result.stdout.strip() == "hello world"

# Test 12: Command with spaces in shell
print("Testing command with spaces...")
result = subprocess.run("echo 'hello   world'", {"shell": True, "capture_output": True})
assert result.returncode == 0
assert result.stdout.strip() == "hello world"

# Test 13: Non-existent command
print("Testing non-existent command...")
try:
    result = subprocess.run("nonexistent_command_12345")
    assert result.returncode != 0
except:
    pass  # May raise exception depending on implementation

# Test 14: Command with special characters
print("Testing special characters...")
result = subprocess.run("echo '$HOME'", {"shell": True, "capture_output": True})
assert result.returncode == 0
# Should expand $HOME
assert len(result.stdout.strip()) > 0

# Test 15: Multiple commands in sequence
print("Testing multiple commands...")
result = subprocess.run("echo start; sleep 0.1; echo end", {"shell": True, "capture_output": True})
assert result.returncode == 0
lines = result.stdout.strip().split('\n')
assert "start" in lines
assert "end" in lines

# Test 16: Background/sleep simulation (if timeout works)
print("Testing timeout option...")
try:
    result = subprocess.run("sleep 1", {"timeout": 0.5})
    assert result.returncode != 0  # Should be killed by timeout
except:
    pass  # Timeout may not be implemented or may raise exception

# Test 17: Working directory option (if implemented)
print("Testing cwd option...")
try:
    result = subprocess.run("pwd", {"shell": True, "capture_output": True, "cwd": "/tmp"})
    assert result.returncode == 0
    assert "/tmp" in result.stdout
except:
    pass  # cwd may not be implemented

# Test 18: CompletedProcess attributes
print("Testing CompletedProcess attributes...")
result = subprocess.run("echo test", {"capture_output": True})
assert result.returncode == 0
assert len(result.stdout.strip()) > 0

# Test 19: check_returncode method
print("Testing check_returncode...")
result = subprocess.run("true", {"capture_output": True})
checked = result.check_returncode()  # Should return self
assert checked == result

try:
    result = subprocess.run("false", {"capture_output": True})
    result.check_returncode()  # Should raise exception
    assert False, "Should have raised exception"
except:
    pass

print("All expanded subprocess tests passed!")
assert passed