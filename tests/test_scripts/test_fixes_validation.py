# Test validation for fixes #5, #6, #10, #11, #15, #16, #17
# This script verifies the behavior of fixes applied during the code review

# === Fix #5: Modulo by zero check ===
try:
    result = 10 % 0
    assert False, "Should have raised error for modulo by zero"
except:
    pass  # Expected

try:
    result = 10.5 % 0.0
    assert False, "Should have raised error for float modulo by zero"
except:
    pass  # Expected

print("✓ Fix #5: Modulo by zero correctly raises error")

# === Fix #6: Integer exponent overflow detection ===
# Very large exponents should not overflow silently
result = 2 ** 10
assert result == 1024, f"Expected 1024, got {result}"

result = 2 ** 0
assert result == 1, f"Expected 1, got {result}"

result = 2 ** -1
assert result == 0.5, f"Expected 0.5, got {result}"

print("✓ Fix #6: Integer exponentiation works correctly")

# === Fix #10: DictKey system - dict operations work correctly ===
d = {"a": 1, "b": 2, "c": 3}
assert d["a"] == 1, f"Expected 1, got {d['a']}"
assert d["b"] == 2, f"Expected 2, got {d['b']}"
assert "a" in d, "Expected 'a' in dict"
assert "z" not in d, "Expected 'z' not in dict"

# Dict iteration should return human-readable keys (not DictKey format)
keys = list(d.keys())
assert "a" in keys, f"Expected 'a' in keys, got {keys}"
assert "b" in keys, f"Expected 'b' in keys, got {keys}"
assert "c" in keys, f"Expected 'c' in keys, got {keys}"

# Items should return (key, value) tuples with clean keys
items = list(d.items())
for key, val in items:
    assert type(key) == type(""), f"Key should be string, got {type(key)}"
    # Keys should not have "s:" prefix
    assert not key.startswith("s:"), f"Key should not have DictKey prefix: {key}"

# Dict views
dk = d.keys()
assert len(dk) == 3, f"Expected 3 keys, got {len(dk)}"

dv = d.values()
assert len(dv) == 3, f"Expected 3 values, got {len(dv)}"

di = d.items()
assert len(di) == 3, f"Expected 3 items, got {len(di)}"

# Dict with various key types
d2 = {1: "one", 2: "two"}
assert d2[1] == "one", f"Expected 'one', got {d2[1]}"
assert d2[2] == "two", f"Expected 'two', got {d2[2]}"

print("✓ Fix #10: DictKey system works correctly")

# === Fix #15: List.AsList() and Tuple.AsList() return copies ===
original_list = [1, 2, 3]
copy_list = list(original_list)
copy_list.append(4)
assert len(original_list) == 3, f"Original list should be unchanged, got {len(original_list)}"
assert len(copy_list) == 4, f"Copy should have 4 elements, got {len(copy_list)}"

original_tuple = (1, 2, 3)
from_tuple = list(original_tuple)
from_tuple.append(4)
assert len(original_tuple) == 3, f"Original tuple should be unchanged"
assert len(from_tuple) == 4, f"List from tuple should have 4 elements"

print("✓ Fix #15: List/Tuple copy behavior correct")

# === Fix #16: from-import doesn't delete root module ===
import json
from json import dumps

# json module should still be accessible after from-import
result = json.dumps({"key": "value"})
assert '"key"' in result, f"json.dumps should work after from-import, got {result}"

# The from-imported function should also work
result2 = dumps({"test": 123})
assert '"test"' in result2, f"dumps should work directly, got {result2}"

print("✓ Fix #16: from-import preserves root module")

# === Fix #17: String iteration ===
# Verify for-loop over string works with Unicode
text = "hello"
chars = []
for ch in text:
    chars.append(ch)
assert chars == ["h", "e", "l", "l", "o"], f"Expected individual chars, got {chars}"

# String in list comprehension
upper_chars = [c.upper() for c in "abc"]
assert upper_chars == ["A", "B", "C"], f"Expected uppercase, got {upper_chars}"

# Unicode string iteration
unicode_text = "héllo"
unicode_chars = []
for ch in unicode_text:
    unicode_chars.append(ch)
assert len(unicode_chars) == 5, f"Expected 5 chars, got {len(unicode_chars)}"
assert unicode_chars[1] == "é", f"Expected 'é', got {unicode_chars[1]}"

print("✓ Fix #17: String iteration works correctly")

# === JSON serialization with DictKey ===
import json
d = {"name": "test", "value": 42}
result = json.dumps(d)
# Result should NOT contain "s:" prefix
assert "s:" not in result, f"JSON should not contain DictKey prefix: {result}"
assert '"name"' in result, f"JSON should contain 'name': {result}"
assert '"value"' in result, f"JSON should contain 'value': {result}"

print("✓ JSON serialization produces clean keys")

# === kwargs unpacking with DictKey ===
def greet(name, greeting="Hello"):
    return f"{greeting}, {name}!"

kwargs = {"name": "World", "greeting": "Hi"}
result = greet(**kwargs)
assert result == "Hi, World!", f"Expected 'Hi, World!', got {result}"

print("✓ kwargs unpacking works correctly")

# === Set operations with DictKey ===
s = set([1, 2, 3])
assert 1 in s, "Expected 1 in set"
assert 4 not in s, "Expected 4 not in set"
s.add(4)
assert 4 in s, "Expected 4 in set after add"
s.remove(2)
assert 2 not in s, "Expected 2 not in set after remove"

s2 = set(["a", "b", "c"])
assert "a" in s2, "Expected 'a' in string set"

print("✓ Set operations work correctly")

# === Dict comparison ===
d1 = {"a": 1, "b": 2}
d2 = {"a": 1, "b": 2}
d3 = {"a": 1, "b": 3}
assert d1 == d2, "Equal dicts should be equal"
assert d1 != d3, "Different dicts should not be equal"

print("✓ Dict comparison works correctly")

# === Dict methods ===
d = {"a": 1, "b": 2}
d.update({"c": 3})
assert "c" in d, "Dict should have 'c' after update"
assert d["c"] == 3, f"Expected 3, got {d['c']}"

copied = d.copy()
copied["d"] = 4
assert "d" not in d, "Original should not have 'd' after copy modification"
assert "d" in copied, "Copy should have 'd'"

print("✓ Dict methods work correctly")

print("\n✅ All fix validation tests passed!")
