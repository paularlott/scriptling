# defaultdict is a real dict with a default factory; reversed() works on dicts
# and their views; OrderedDict.move_to_end / popitem(last=...) work; the
# zero-argument constructors int(), float() and str() give 0, 0.0 and ''.
from collections import defaultdict, OrderedDict

# --- zero-argument constructors (the defaultdict factories) ---
assert int() == 0 and float() == 0.0 and str() == "" and list() == [] and dict() == {}

# --- defaultdict: creation on read, not on get/in/pop ---
dd = defaultdict(list)
dd["z"].append(1)
dd["a"].append(2)
assert list(dd) == ["z", "a"] and dict(dd) == {"z": [1], "a": [2]}
assert list(dd.items()) == [("z", [1]), ("a", [2])]
assert list(dd.keys()) == ["z", "a"] and list(dd.values()) == [[1], [2]]
assert len(dd) == 2 and "a" in dd and "q" not in dd and len(dd) == 2
assert dd.get("x") is None and dd.get("z") == [1] and len(dd) == 2
assert dd["q"] == [] and len(dd) == 3 and "q" in dd
assert dd.pop("z") == [1] and list(dd) == ["a", "q"]
assert dd.popitem() == ("q", [])
dd.update({"k": [9]})
assert list(dd) == ["a", "k"] and dd.setdefault("m", [5]) == [5]
del dd["m"]
assert sorted(dd) == ["a", "k"] and min(dd) == "a" and max(dd) == "k"
assert dd == {"a": [2], "k": [9]}

# copies keep the factory; dict(dd) and {**dd} are plain dicts
c = dd.copy()
c["new"].append(1)
assert len(c) == 3 and len(dd) == 2
assert isinstance(c, dict) and isinstance(c, defaultdict) and not isinstance({}, defaultdict)
plain = dict(dd)
try:
    plain["absent"]
    assert False, "dict(dd) must be a plain dict"
except KeyError:
    pass
import copy
cc = copy.copy(dd)
cd = copy.deepcopy(dd)
cc["x"].append(1)
cd["y"].append(1)
assert len(cc) == 3 and len(cd) == 3 and len(dd) == 2
import json
assert json.dumps({"a": defaultdict(int, {"x": 1})}) == '{"a": {"x": 1}}'

# --- factories: types, lambdas, functions, nesting ---
d0 = defaultdict(lambda: 0)
d0["a"] += 1
d0["b"] += 2
assert dict(d0) == {"a": 1, "b": 2}
d1 = defaultdict(set)
d1["a"].add(1)
d1["a"].add(2)
assert d1["a"] == {1, 2}
d2 = defaultdict(lambda: defaultdict(int))
d2["a"]["b"] += 1
d2["a"]["c"] += 2
d2["x"]["y"] += 3
assert d2["a"]["b"] == 1 and dict(d2["a"]) == {"b": 1, "c": 2} and len(d2) == 2
d3 = defaultdict(lambda: [0, 0])
d3["a"][0] += 1
assert d3["a"] == [1, 0]
d4 = defaultdict(str)
d4["a"] += "x"
assert d4["a"] == "x"
d5 = defaultdict(float)
d5["a"] += 1.5
assert d5["a"] == 1.5

def make_default():
    return "dflt"
d6 = defaultdict(make_default)
assert d6["q"] == "dflt" and len(d6) == 1

d7 = defaultdict()
try:
    d7["q"]
    assert False, "no factory: KeyError"
except KeyError:
    pass
assert defaultdict(None).default_factory is None and defaultdict(list).default_factory is not None

# keys keep their types (1, "1" and (1, 2) are distinct)
d8 = defaultdict(int)
d8[1] += 1
d8["1"] += 5
d8[(1, 2)] += 1
assert dict(d8) == {1: 1, "1": 5, (1, 2): 1}

# initial contents
d9 = defaultdict(int, {"a": 1}, b=2)
assert d9 == {"a": 1, "b": 2} and d9["zz"] == 0
d10 = defaultdict(list, [("x", [1]), ("y", [2])])
assert d10 == {"x": [1], "y": [2]}
for bad in (5, "x"):
    try:
        defaultdict(bad)
        assert False, "expected TypeError"
    except TypeError:
        pass

# the canonical idioms
counts = defaultdict(int)
for w in "a b a c b a".split():
    counts[w] += 1
assert sorted(counts.items(), key=lambda kv: -kv[1]) == [("a", 3), ("b", 2), ("c", 1)]
groups = defaultdict(list)
for k, v in [("x", 1), ("y", 2), ("x", 3)]:
    groups[k].append(v)
seen = []
for k, vs in groups.items():
    seen.append((k, vs))
assert seen == [("x", [1, 3]), ("y", [2])]
a, b = groups
assert (a, b) == ("x", "y") and [*groups] == ["x", "y"] and {**groups} == {"x": [1, 3], "y": [2]}

# --- reversed() on dicts and views ---
d = {"a": 1, "b": 2, "c": 3}
d["z"] = 0
del d["b"]
d["b"] = 9
assert list(reversed(d)) == ["b", "z", "c", "a"]
assert list(reversed(d.items())) == [("b", 9), ("z", 0), ("c", 3), ("a", 1)]
assert list(reversed(d.keys())) == ["b", "z", "c", "a"]
assert list(reversed(d.values())) == [9, 0, 3, 1]
assert list(reversed({})) == [] and next(reversed(d)) == "b"
assert list(reversed(defaultdict(int, {"x": 1, "y": 2}))) == ["y", "x"]
try:
    reversed(5)
    assert False, "expected TypeError"
except TypeError:
    pass

# --- OrderedDict: move_to_end, popitem(last=...), kwargs ---
od = OrderedDict([("a", 1), ("b", 2), ("c", 3)])
od.move_to_end("a")
assert list(od) == ["b", "c", "a"]
od.move_to_end("a", last=False)
assert list(od) == ["a", "b", "c"]
od.move_to_end("b", False)
assert list(od) == ["b", "a", "c"]
od.move_to_end("b", last=True)
od.move_to_end("b")  # already last: unchanged
assert list(od) == ["a", "c", "b"]
assert od.popitem(last=False) == ("a", 1)
assert od.popitem() == ("b", 2)
assert od.popitem(last=True) == ("c", 3)
try:
    od.popitem()
    assert False, "empty popitem: KeyError"
except KeyError:
    pass
try:
    od.move_to_end("missing")
    assert False, "move_to_end of a missing key: KeyError"
except KeyError:
    pass
assert sorted(OrderedDict(b=1, a=2)) == ["a", "b"]
assert len(OrderedDict(x=1)) == 1 and OrderedDict([("k", 1)])["k"] == 1

# the LRU-cache idiom
class LRU:
    def __init__(self, cap):
        self.cap = cap
        self.data = OrderedDict()
    def get(self, key):
        if key not in self.data:
            return None
        self.data.move_to_end(key)
        return self.data[key]
    def put(self, key, value):
        if key in self.data:
            self.data.move_to_end(key)
        self.data[key] = value
        if len(self.data) > self.cap:
            self.data.popitem(last=False)

cache = LRU(2)
cache.put("a", 1)
cache.put("b", 2)
assert cache.get("a") == 1
cache.put("c", 3)
assert cache.get("b") is None and cache.get("a") == 1 and cache.get("c") == 3
assert list(cache.data) == ["a", "c"]
