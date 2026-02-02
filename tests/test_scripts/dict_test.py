person = {"name": "Alice", "age": 30}
assert person["name"] == "Alice"
assert person["age"] == 30

# Test dict methods
d = {"a": 1, "b": 2, "c": 3}
assert d.get("a") == 1
assert d.get("x") == None
assert d.get("x", "default") == "default"

d = {"a": 1, "b": 2, "c": 3}
popped = d.pop("b")
assert popped == 2

popped_default = d.pop("x", "not found")
assert popped_default == "not found"

d1 = {"a": 1, "b": 2}
d2 = {"b": 20, "c": 3}
d1.update(d2)
assert d1["b"] == 20
assert d1["c"] == 3

d = {"a": 1, "b": 2}
d.clear()
assert len(d.keys()) == 0

original = {"a": 1, "b": 2}
copied = original.copy()
copied["c"] = 3
assert len(original.keys()) == 2
assert copied["c"] == 3

d = {"a": 1}
result = d.setdefault("a", 100)
assert result == 1
result = d.setdefault("b", 200)
assert result == 200
assert d["b"] == 200

# Test dict.fromkeys()
d = {}.fromkeys(["a", "b", "c"])
assert "a" in d
assert d["a"] == None

d = {}.fromkeys(["x", "y", "z"], 0)
assert d["x"] == 0
assert d["y"] == 0
assert d["z"] == 0

d = {}.fromkeys("abc")
assert "a" in d
assert "b" in d
assert "c" in d

d = {"a": {"b": {"c": 1}}, "x": [1, 2, {"y": 3}]}
assert d["a"]["b"]["c"] == 1
assert d["x"][2]["y"] == 3