# Basic boolean operators
assert True and True
assert not (True and False)
assert True or False
assert not (False or False)
assert not False

# Short-circuit evaluation
x = 0
result = False and (1 / x)  # should not raise
assert result == False

called = False
def side_effect():
    global called
    called = True
    return True

result = True or side_effect()  # side_effect should not be called
assert result == True
assert called == False

result = False and side_effect()
assert result == False
assert called == False

# Truthiness of non-booleans
assert not 0
assert 1
assert not ""
assert "x"
assert not []
assert [0]
assert not {}
assert {"k": 1}
assert not None

# bool() conversion
assert bool(0) == False
assert bool(1) == True
assert bool("") == False
assert bool("x") == True
assert bool([]) == False
assert bool([1]) == True
assert bool(None) == False

# and/or return values (not just True/False)
assert (1 and 2) == 2
assert (0 and 2) == 0
assert (1 or 2) == 1
assert (0 or 2) == 2
