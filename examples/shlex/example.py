import shlex
import subprocess

# --- quote(): safe command-line construction ---
user_input = "hello; rm -rf /"
safe = shlex.quote(user_input)
print(f"quote({user_input!r}) = {safe!r}")

# --- split(): parse a shell command string ---
cmd_line = 'grep --color=auto "hello world" *.py'
tokens = shlex.split(cmd_line)
print(f"\nsplit({cmd_line!r}) =")
for i, tok in enumerate(tokens):
    print(f"  [{i}] {tok}")

# --- join(): rebuild a command from args ---
args = ["rsync", "-avz", "My Documents/", "user@host:/backup/"]
reconstructed = shlex.join(args)
print(f"\njoin({args}) = {reconstructed}")

# --- Practical use: safe subprocess invocation ---
# Instead of string concatenation (vulnerable to injection), use split + subprocess
filename = "my file (1).txt"
command = "cat " + shlex.join([filename])
print(f"\nSafe command: {command}")

# Verify the round-trip: split(join(args)) == args
round_tripped = shlex.split(command)
print(f"Round-trip: {round_tripped}")
assert round_tripped == ["cat", filename]
print("Round-trip verified!")
