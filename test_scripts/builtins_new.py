# Test new builtins: repr, hash, id, format, hasattr, getattr, setattr

# Test repr
result = repr("hello")
assert result == "'hello'", "repr() on string should add quotes"

result = repr(42)
assert result == "42", "repr() on int should return string"

result = repr([1, 2, 3])
assert result == "[1, 2, 3]", "repr() on list"

# Test hash
h1 = hash("hello")
h2 = hash("hello")
assert h1 == h2, "hash() should be consistent"

h3 = hash("world")
assert h1 != h3, "hash() should differ for different values"

# Test id
x = [1, 2, 3]
id1 = id(x)
id2 = id(x)
# id should return something (can't guarantee exact value)
assert type(id1) == "INTEGER", "id() should return integer"

# Test format
result = format(42)
assert result == "42", "format() with no spec"

result = format(42, "d")
assert result == "42", "format() with d spec"

result = format(255, "x")
assert result == "ff", "format() with hex spec"

result = format(255, "X")
assert result == "FF", "format() with upper hex spec"

result = format(8, "b")
assert result == "1000", "format() with binary spec"

result = format(3.14159, ".2f")
assert result == "3.14", "format() with float precision"

result = format(0.5, "%")
assert result == "50.00%", "format() with percent"

result = format("hello", ">10")
assert result == "     hello", "format() right align"

result = format("hello", "<10")
assert result == "hello     ", "format() left align"

result = format("hello", "^11")
assert result == "   hello   ", "format() center align"

# Test hasattr, getattr, setattr with dict
d = {"name": "Alice", "age": 30}

result = hasattr(d, "name")
assert result == True, "hasattr() should find existing key"

result = hasattr(d, "missing")
assert result == False, "hasattr() should not find missing key"

result = getattr(d, "name")
assert result == "Alice", "getattr() should get value"

result = getattr(d, "missing", "default")
assert result == "default", "getattr() should return default"

setattr(d, "city", "NYC")
assert d["city"] == "NYC", "setattr() should set value"

print("All new builtin tests passed!")

# Return true for test framework
True
