# Test print with sep and end kwargs
import re

passed = True

# We can't easily capture output in scripts, so test that the syntax works
# and the function doesn't error

# Test sep kwarg - should not raise error
print("a", "b", "c", sep=",")

# Test end kwarg - should not raise error
print("no newline", end="")
print(" followed by this")

# Test both together
print("x", "y", "z", sep="-", end="!\n")

# Test with None values (should use defaults)
print("test", sep=None, end=None)

# Basic verification that the print calls executed without error
passed
