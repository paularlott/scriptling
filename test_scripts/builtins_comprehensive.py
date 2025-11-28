# Test comprehensive builtin functions
failures = 0

# enumerate
items = ["a", "b", "c"]
enum_result = enumerate(items)
if len(enum_result) != 3:
    failures += 1
# Each element should be [index, value]
if enum_result[0][0] != 0:
    failures += 1
if enum_result[0][1] != "a":
    failures += 1

# zip
a = [1, 2, 3]
b = ["x", "y", "z"]
zipped = zip(a, b)
if len(zipped) != 3:
    failures += 1
if zipped[0][0] != 1:
    failures += 1
if zipped[0][1] != "x":
    failures += 1

# any / all
if not any([False, True, False]):
    failures += 1
if not all([True, True, True]):
    failures += 1
if all([True, False, True]):
    failures += 1

# bool
if bool(0):
    failures += 1
if not bool(1):
    failures += 1
if bool(""):
    failures += 1
if not bool("hello"):
    failures += 1

# abs
if abs(-5) != 5:
    failures += 1
if abs(5) != 5:
    failures += 1

# min / max
if min(3, 1, 2) != 1:
    failures += 1
if max(3, 1, 2) != 3:
    failures += 1

# round
if round(3.7) != 4:
    failures += 1

# chr / ord
if chr(65) != "A":
    failures += 1
if ord("A") != 65:
    failures += 1

# reversed
rev = reversed([1, 2, 3])
if rev[0] != 3:
    failures += 1
if rev[2] != 1:
    failures += 1

# map
doubled = map(lambda x: x * 2, [1, 2, 3])
if doubled[0] != 2:
    failures += 1
if doubled[1] != 4:
    failures += 1

# filter
evens = filter(lambda x: x % 2 == 0, [1, 2, 3, 4, 5])
if len(evens) != 2:
    failures += 1
if evens[0] != 2:
    failures += 1

# list from string
chars = list("abc")
if len(chars) != 3:
    failures += 1
if chars[0] != "a":
    failures += 1

# dict
d = dict()
if len(d.keys()) != 0:
    failures += 1

# tuple
t = tuple([1, 2, 3])
if len(t) != 3:
    failures += 1

# set (creates list of unique values)
s = set([1, 2, 2, 3, 3, 3])
if len(s) != 3:
    failures += 1

failures == 0
