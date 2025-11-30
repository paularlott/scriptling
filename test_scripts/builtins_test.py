# Test builtin functions

# Test hex(), bin(), oct() builtins
assert hex(255) == "0xff"
assert hex(16) == "0x10"
assert hex(0) == "0x0"
assert hex(-255) == "-0xff"
assert hex(1000) == "0x3e8"

assert bin(10) == "0b1010"
assert bin(255) == "0b11111111"
assert bin(0) == "0b0"
assert bin(-10) == "-0b1010"
assert bin(1) == "0b1"

assert oct(8) == "0o10"
assert oct(64) == "0o100"
assert oct(0) == "0o0"
assert oct(-8) == "-0o10"
assert oct(255) == "0o377"

# Test enumerate
items = ["a", "b", "c"]
enum_result = enumerate(items)
assert len(enum_result) == 3
assert enum_result[0][0] == 0
assert enum_result[0][1] == "a"

# Test zip
a = [1, 2, 3]
b = ["x", "y", "z"]
zipped = zip(a, b)
assert len(zipped) == 3
assert zipped[0][0] == 1
assert zipped[0][1] == "x"

# Test any / all
assert any([False, True, False])
assert all([True, True, True])
assert not all([True, False, True])

# Test bool
assert not bool(0)
assert bool(1)
assert not bool("")
assert bool("hello")

# Test abs
assert abs(-5) == 5
assert abs(5) == 5

# Test min / max
assert min(3, 1, 2) == 1
assert max(3, 1, 2) == 3

# Test round
assert round(3.7) == 4

# Test chr / ord
assert chr(65) == "A"
assert ord("A") == 65

# Test reversed
rev = reversed([1, 2, 3])
assert rev[0] == 3
assert rev[2] == 1

# Test map
doubled = map(lambda x: x * 2, [1, 2, 3])
assert doubled[0] == 2
assert doubled[1] == 4

# Test filter
evens = filter(lambda x: x % 2 == 0, [1, 2, 3, 4, 5])
assert len(evens) == 2
assert evens[0] == 2

# Test list from string
chars = list("abc")
assert len(chars) == 3
assert chars[0] == "a"

# Test dict
d = dict()
assert len(d.keys()) == 0

# Test tuple
t = tuple([1, 2, 3])
assert len(t) == 3

# Test set (creates list of unique values)
s = set([1, 2, 2, 3, 3, 3])
assert len(s) == 3

# Test repr
result = repr("hello")
assert result == "'hello'"

result = repr(42)
assert result == "42"

result = repr([1, 2, 3])
assert result == "[1, 2, 3]"

# Test hash
h1 = hash("hello")
h2 = hash("hello")
assert h1 == h2

h3 = hash("world")
assert h1 != h3

# Test id
x = [1, 2, 3]
id1 = id(x)
id2 = id(x)
assert type(id1) == "INTEGER"

# Test format
result = format(42)
assert result == "42"

result = format(42, "d")
assert result == "42"

result = format(255, "x")
assert result == "ff"

result = format(255, "X")
assert result == "FF"

result = format(8, "b")
assert result == "1000"

result = format(3.14159, ".2f")
assert result == "3.14"

result = format(0.5, "%")
assert result == "50.00%"

result = format("hello", ">10")
assert result == "     hello"

result = format("hello", "<10")
assert result == "hello     "

result = format("hello", "^11")
assert result == "   hello   "

# Test hasattr, getattr, setattr with dict
d = {"name": "Alice", "age": 30}

result = hasattr(d, "name")
assert result == True

result = hasattr(d, "missing")
assert result == False

result = getattr(d, "name")
assert result == "Alice"

result = getattr(d, "missing", "default")
assert result == "default"

setattr(d, "city", "NYC")
assert d["city"] == "NYC"