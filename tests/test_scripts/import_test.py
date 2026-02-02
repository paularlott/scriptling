# Test import statement functionality
print("Testing import statements...")

# Test 1: Single import
try:
    import math
    print("Single import works")
except Exception as e:
    print(f"Single import failed: {e}")

# Test 2: Multiple imports on one line
try:
    import json, re
    print("Multiple imports work")
except Exception as e:
    print(f"Multiple imports failed: {e}")

print("Import statement tests completed!")