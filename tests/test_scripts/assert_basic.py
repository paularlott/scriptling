# Test assert statement

# Basic truthy assertions
assert True
assert 1 == 1
assert 1 != 2
assert not False

# Numeric comparisons
x = 10
assert x > 5
assert x < 100
assert x >= 10
assert x <= 10
assert x == 10

# String assertions
assert len("hello") == 5
assert "hello" == "hello"
assert "hello" != "world"
assert "ell" in "hello"

# Truthy collections
assert [1, 2, 3]
assert {"key": "value"}
assert (1, 2)
assert {1, 2, 3}

# Falsy checks via not
assert not []
assert not {}
assert not ""
assert not 0
assert not None

# Assert with message (passing — message should not appear)
assert True, "this should not appear"
assert x > 0, "x must be positive"
assert x == 10, f"expected 10, got {x}"

# Assert with complex expressions
assert 2 ** 8 == 256
assert [i for i in range(3)] == [0, 1, 2]
assert max([3, 1, 2]) == 3

# Assert inside a function
def check(val):
    assert val > 0, "must be positive"
    return val * 2

assert check(5) == 10

# Assert inside a loop
for i in range(1, 4):
    assert i > 0

# Assert with boolean operators
assert True and True
assert True or False
assert not (False and True)

print("✓ All assertions passed")
