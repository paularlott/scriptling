# Test dict methods
failures = 0

# get
d = {"a": 1, "b": 2, "c": 3}
if d.get("a") != 1:
    failures += 1
if d.get("x") != None:
    failures += 1
if d.get("x", "default") != "default":
    failures += 1

# pop
d = {"a": 1, "b": 2, "c": 3}
popped = d.pop("b")
if popped != 2:
    failures += 1

popped_default = d.pop("x", "not found")
if popped_default != "not found":
    failures += 1

# update
d1 = {"a": 1, "b": 2}
d2 = {"b": 20, "c": 3}
d1.update(d2)
if d1["b"] != 20:
    failures += 1
if d1["c"] != 3:
    failures += 1

# clear
d = {"a": 1, "b": 2}
d.clear()
if len(d.keys()) != 0:
    failures += 1

# copy
original = {"a": 1, "b": 2}
copied = original.copy()
# Use setdefault to add key instead of index assignment
copied.setdefault("c", 3)
if len(original.keys()) != 2:
    failures += 1
if copied["c"] != 3:
    failures += 1

# setdefault
d = {"a": 1}
result = d.setdefault("a", 100)
if result != 1:
    failures += 1

result = d.setdefault("b", 200)
if result != 200:
    failures += 1
if d["b"] != 200:
    failures += 1

failures == 0
