# Test dict.fromkeys()

# Create from list
d = {}.fromkeys(["a", "b", "c"])
assert "a" in d, "fromkeys creates keys"
assert d["a"] == None, "default value is None"

# Create from list with value
d = {}.fromkeys(["x", "y", "z"], 0)
assert d["x"] == 0, "fromkeys with default value"
assert d["y"] == 0, "fromkeys with default value - y"
assert d["z"] == 0, "fromkeys with default value - z"

# Create from string
d = {}.fromkeys("abc")
assert "a" in d, "fromkeys from string"
assert "b" in d, "fromkeys from string - b"
assert "c" in d, "fromkeys from string - c"

# Create from tuple
d = {}.fromkeys((1, 2, 3), "default")
assert d[1] == "default", "fromkeys from tuple"

print("All dict.fromkeys() tests passed!")

# Return true for test framework
True
