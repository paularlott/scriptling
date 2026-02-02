#!/usr/bin/env scriptling
# Test os.path.getmtime function

import os.path
import time

# Create a test file
test_file = "/tmp/scriptling_mtime_test.txt"
os.write_file(test_file, "test content")

# Get modification time
mtime = os.path.getmtime(test_file)
assert type(mtime) == type(1.0), "getmtime should return float"
assert mtime > 0, "mtime should be positive"

# Convert to readable format
readable = time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(mtime))
assert len(readable) > 0, "readable time should not be empty"

print("✓ os.path.getmtime returns float timestamp")
print("✓ time.strftime converts timestamp to readable format")
print(f"  Example: {readable}")

# Clean up
os.remove(test_file)

print("\n✅ os.path.getmtime test passed")
