# Test general assignment expressions (e.g., dict[key] = value, list[index] = value)
fails = 0

# Test 1: Dict key assignment
d = {}
d["key"] = "value"
if d["key"] != "value":
    fails = fails + 1

# Test 2: Dict key assignment with variable key
d = {}
key = "mykey"
d[key] = 123
if d["mykey"] != 123:
    fails = fails + 1

# Test 3: List index assignment
lst = [1, 2, 3]
lst[0] = 10
if lst[0] != 10:
    fails = fails + 1

# Test 4: List negative index assignment
lst = [1, 2, 3]
lst[-1] = 30
if lst[2] != 30:
    fails = fails + 1

# Test 5: Nested dict assignment
d = {"a": {"b": 1}}
d["a"]["b"] = 2
if d["a"]["b"] != 2:
    fails = fails + 1

# Test 6: Dict with integer keys
d = {}
d[1] = "one"
d[2] = "two"
if d[1] != "one":
    fails = fails + 1

# Test 7: Assignment in loop
d = {}
for i in range(3):
    d[i] = i * 2
if d[0] != 0 or d[1] != 2 or d[2] != 4:
    fails = fails + 1

# Test 8: Multiple assignments
lst = [0, 0, 0]
lst[0] = 1
lst[1] = 2
lst[2] = 3
if lst != [1, 2, 3]:
    fails = fails + 1

# Test 9: Building dict from loop
unique = {}
items = [("a", 1), ("b", 2), ("a", 3)]
for key, val in items:
    if key not in unique:
        unique[key] = val
if len(unique) != 2:
    fails = fails + 1

fails == 0
