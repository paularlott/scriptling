# int(x, base) tests

# hex
assert int("ff", 16) == 255, f"expected 255, got {int('ff', 16)}"
assert int("FF", 16) == 255, f"expected 255, got {int('FF', 16)}"
assert int("0xff", 16) == 255, f"expected 255, got {int('0xff', 16)}"
assert int("1a2b", 16) == 6699, f"expected 6699, got {int('1a2b', 16)}"

# binary
assert int("1010", 2) == 10, f"expected 10, got {int('1010', 2)}"
assert int("0b1010", 2) == 10, f"expected 10, got {int('0b1010', 2)}"
assert int("11111111", 2) == 255, f"expected 255, got {int('11111111', 2)}"

# octal
assert int("77", 8) == 63, f"expected 63, got {int('77', 8)}"
assert int("0o77", 8) == 63, f"expected 63, got {int('0o77', 8)}"

# base 10 explicit
assert int("42", 10) == 42, f"expected 42, got {int('42', 10)}"

# base 36
assert int("z", 36) == 35, f"expected 35, got {int('z', 36)}"

# whitespace trimmed
assert int("  ff  ", 16) == 255, f"expected 255 with whitespace, got {int('  ff  ', 16)}"

# single-arg still works
assert int("123") == 123, f"expected 123, got {int('123')}"
assert int(3.9) == 3, f"expected 3, got {int(3.9)}"
assert int(42) == 42, f"expected 42, got {int(42)}"

# invalid string for base raises error
try:
    int("xyz", 10)
    assert False, "should have raised"
except Exception as e:
    assert "cannot convert" in str(e), f"unexpected error: {e}"

# base out of range raises error
try:
    int("10", 1)
    assert False, "should have raised"
except Exception as e:
    assert "base must be" in str(e), f"unexpected error: {e}"

# float with base raises error
try:
    int(3.14, 16)
    assert False, "should have raised"
except Exception as e:
    pass

print("All int(x, base) tests passed")
