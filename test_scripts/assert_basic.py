# Test assert statement

# Basic assert - should pass
assert True
assert 1 == 1
assert len("hello") == 5

# Test assert with expression
x = 10
assert x > 5
assert x < 100

# Test list/dict truthy assertions
assert [1, 2, 3]
assert {"key": "value"}

print("✓ All assertions passed")
