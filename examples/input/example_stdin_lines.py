import sys

# Read stdin line by line
# Usage: echo -e "hello\nworld" | scriptling example_stdin_lines.py
for line in sys.stdin:
    print("Got:", line.rstrip())
