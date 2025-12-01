# Test attribute access functions (getattr, setattr, hasattr, delattr)

# Test with class instances
class Person:
    def __init__(self, name, age):
        self.name = name
        self.age = age

p = Person("Alice", 30)

# Test hasattr
assert hasattr(p, "name") == True
assert hasattr(p, "age") == True
assert hasattr(p, "email") == False

# Test getattr
assert getattr(p, "name") == "Alice"
assert getattr(p, "age") == 30
assert getattr(p, "email", "no-email") == "no-email"

# Test setattr
setattr(p, "email", "alice@example.com")
assert p.email == "alice@example.com"
assert hasattr(p, "email") == True

setattr(p, "age", 31)
assert p.age == 31

# Test delattr
delattr(p, "email")
assert hasattr(p, "email") == False

# Test with dictionaries
d = {"a": 1, "b": 2}

assert hasattr(d, "a") == True
assert hasattr(d, "c") == False

assert getattr(d, "a") == 1
assert getattr(d, "c", "default") == "default"

setattr(d, "c", 3)
assert d["c"] == 3

delattr(d, "c")
assert hasattr(d, "c") == False

# Test getattr with no default (should raise error)
try:
    getattr(p, "nonexistent")
    assert False, "Should have raised error"
except:
    pass  # Expected

print("All attribute access tests passed!")
