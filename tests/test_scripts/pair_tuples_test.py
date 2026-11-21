# Pairs from enumerate, zip and dict.items() are ordinary tuples.
names = ["a", "b", "c"]
pairs = list(enumerate(names))
assert pairs == [(0, "a"), (1, "b"), (2, "c")]
p = pairs[1]
assert isinstance(p, tuple) and len(p) == 2
assert p[0] == 1 and p[-1] == "b"
assert p + (9,) == (1, "b", 9)
assert p == (1, "b")
assert p * 2 == (1, "b", 1, "b")
assert pairs[1] == (1, "b"), "building a longer tuple from a pair leaves the pair alone"
assert {p: "x"}[(1, "b")] == "x"
assert sorted(enumerate(["z", "y"]), key=lambda t: t[1]) == [(1, "y"), (0, "z")]
assert list(enumerate(names, start=5))[0] == (5, "a")

zipped = list(zip(names, [1, 2, 3]))
assert zipped == [("a", 1), ("b", 2), ("c", 3)]
assert dict(zipped) == {"a": 1, "b": 2, "c": 3}
assert list(zip(names, names, names))[0] == ("a", "a", "a")
assert list(zip(names)) == [("a",), ("b",), ("c",)]
a, b = zipped[2]
assert (a, b) == ("c", 3)
x, *rest = zipped[0]
rest.append("more")
assert zipped[0] == ("a", 1)

d = {"k1": 1, "k2": 2}
items = list(d.items())
assert items == [("k1", 1), ("k2", 2)]
d["k1"] = 100
assert items[0] == ("k1", 1), "an items() pair is a snapshot of the value"
for k, v in d.items():
    assert d[k] == v
for i, (k, v) in enumerate(d.items()):
    assert (i, k, v) in [(0, "k1", 100), (1, "k2", 2)]
held = [pair for pair in enumerate("xyz")]
assert held == [(0, "x"), (1, "y"), (2, "z")]
print("OK")
