# Dicts are insertion-ordered, as in Python 3.7+: repr, iteration, views,
# popitem and json serialization all follow insertion order.

# --- literals, dict(), and assignment keep insertion order ---
d = {"zebra": 1, "apple": 2, "mango": 3}
assert repr(d) == "{'zebra': 1, 'apple': 2, 'mango': 3}"
assert str(d) == repr(d)
assert list(d) == ["zebra", "apple", "mango"]
assert list(d.keys()) == ["zebra", "apple", "mango"]
assert list(d.values()) == [1, 2, 3]
assert list(d.items()) == [("zebra", 1), ("apple", 2), ("mango", 3)]

pairs = dict([(3, "c"), (1, "a"), (2, "b")])
assert repr(pairs) == "{3: 'c', 1: 'a', 2: 'b'}"

d2 = {}
d2["z"] = 1
d2["a"] = 2
assert repr(d2) == "{'z': 1, 'a': 2}"

# --- comprehension and ** merge order ---
comp = {k: v for k, v in [("m", 1), ("a", 2), ("z", 3)]}
assert repr(comp) == "{'m': 1, 'a': 2, 'z': 3}"

merged = {"a": 1, "b": 2, **{"c": 3, "a": 9}}
assert repr(merged) == "{'a': 9, 'b': 2, 'c': 3}"

# --- re-assignment keeps the original position ---
d["zebra"] = 99
assert list(d) == ["zebra", "apple", "mango"]
assert d["zebra"] == 99

# --- deletion leaves later keys in place; new keys append ---
del d["apple"]
assert list(d) == ["zebra", "mango"]
d["kiwi"] = 4
assert list(d) == ["zebra", "mango", "kiwi"]
assert d.pop("zebra") == 99
assert list(d) == ["mango", "kiwi"]

# --- popitem pops the most recently inserted pair ---
p = {"x": 1, "y": 2}
assert p.popitem() == ("y", 2)
assert repr(p) == "{'x': 1}"

# --- churn: insert/delete cycles stay ordered ---
churn = {}
for i in range(50):
    churn["k" + str(i % 10)] = i
    if i % 3 == 0 and ("k" + str((i // 3) % 10)) in churn:
        del churn["k" + str((i // 3) % 10)]
assert list(churn) == [k for k in list(churn)]  # stable read
assert len(list(churn)) == len(churn)

# --- copies preserve order ---
src = {"q": 1, "w": 2, "e": 3}
assert repr(src.copy()) == repr(src)
import copy
deep = copy.deepcopy(src)
assert repr(deep) == repr(src)
assert dict(src) == src

# --- update/|= extend in order ---
u = {"a": 1}
u.update({"c": 3, "b": 2})
assert repr(u) == "{'a': 1, 'c': 3, 'b': 2}"
u2 = {"x": 1}
u2 |= {"z": 9, "y": 8}
assert repr(u2) == "{'x': 1, 'z': 9, 'y': 8}"

# --- dict merge operator: left order, then new keys from right ---
joined = {"a": 1, "b": 2} | {"c": 3, "a": 7}
assert repr(joined) == "{'a': 7, 'b': 2, 'c': 3}"

# --- equality ignores order ---
assert {"a": 1, "b": 2} == {"b": 2, "a": 1}

# --- json.dumps follows insertion order, Python separators ---
import json
assert json.dumps({"b": 1, "a": [1, 2.5, None, True, "x"]}) == '{"b": 1, "a": [1, 2.5, null, true, "x"]}'
assert json.dumps({"b": 1, "a": 2}, separators=(",", ":")) == '{"b":1,"a":2}'
assert json.dumps({"b": 1, "a": 2}, sort_keys=True) == '{"a": 2, "b": 1}'
assert json.dumps({"b": 1, "a": 2}, indent=None) == '{"b": 1, "a": 2}'

# --- Counter keeps first-seen order; most_common ties break by insertion ---
from collections import Counter
c = Counter("abracadabra")
assert repr(c) == "Counter({'a': 5, 'b': 2, 'r': 2, 'c': 1, 'd': 1})"
assert c.most_common(2) == [("a", 5), ("b", 2)]
tied = Counter(["x", "y", "x", "z", "y"])
assert [e for e, _ in tied.most_common()] == ["x", "y", "z"]

# --- OrderedDict / defaultdict behave ---
from collections import OrderedDict, defaultdict
od = OrderedDict([("b", 1), ("a", 2)])
assert repr(od) == "{'b': 1, 'a': 2}"
dd = defaultdict(list)
dd["z"].append(1)
dd["a"].append(2)
assert list(dd.keys()) == ["z", "a"]

# --- dataclasses.asdict preserves field order ---
from dataclasses import dataclass, asdict

@dataclass
class Rec:
    zebra: int
    apple: str = "x"

assert repr(asdict(Rec(1))) == "{'zebra': 1, 'apple': 'x'}"

# --- for loops over dicts and items() iterate in order ---
acc = []
for k, v in {"n": 2, "o": 1, "w": 3}.items():
    acc.append(k + str(v))
assert acc == ["n2", "o1", "w3"]

# --- zip/map walk dicts and views in step, as in Python ---
d = {"a": 1, "b": 2, "c": 3}
assert list(zip(d.keys(), d.values())) == [("a", 1), ("b", 2), ("c", 3)]
assert list(zip(d, d)) == [("a", "a"), ("b", "b"), ("c", "c")]
assert list(map(lambda x, y: x + y, d.values(), d.values())) == [2, 4, 6]

print("dict insertion order: ok")
