import sys

# Read all stdin at once, or use input() to read one line
# Usage: echo "hello world" | scriptling example_stdin_input.py

# Read a single line with input()
line = input()
print("input() got:", line)

# Read remaining data with sys.stdin.read()
# data = sys.stdin.read()
# print("read() got:", data)

# Read one line with sys.stdin.readline()
# line = sys.stdin.readline()
# print("readline() got:", line.rstrip())
