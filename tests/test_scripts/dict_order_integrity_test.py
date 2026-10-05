# Insertion order stays consistent across every way of mutating a dict:
# Counter pop/clear then re-add, deletes at scale, Go-built dicts that are
# mutated afterwards, **kwargs, and json round trips.
from collections import Counter

# --- Counter: pop / del / clear followed by re-adding the key ---
c = Counter("aabbc")
c.pop("a")
c["a"] = 5
assert list(c) == ["b", "c", "a"] and len(c) == 3
assert dict(c) == {"b": 2, "c": 1, "a": 5}

c = Counter("aab")
c.pop("a")
c.update("ab")
assert list(c) == ["b", "a"] and c["a"] == 1 and c["b"] == 2

c = Counter("aab")
del c["a"]
c["a"] = 1
assert list(c) == ["b", "a"] and len(c) == 2

c = Counter("aab")
c.clear()
c["z"] = 1
c["a"] = 2
assert list(c) == ["z", "a"] and len(c) == 2

c = Counter("aabbbc")
assert c.most_common() == [("b", 3), ("a", 2), ("c", 1)]

# --- json.loads keeps document order at every level, and later edits append ---
import json
doc = '{"zeta": 1, "alpha": {"y": 1, "b": [{"q": 1, "a": 2}]}, "mid": 3, "beta": 4, "omega": 5, "chi": 6, "psi": 7}'
loaded = json.loads(doc)
assert list(loaded) == ["zeta", "alpha", "mid", "beta", "omega", "chi", "psi"]
assert list(loaded["alpha"]) == ["y", "b"]
assert list(loaded["alpha"]["b"][0]) == ["q", "a"]
loaded["new"] = 1
loaded["alpha"]["added"] = 2
assert list(loaded)[-1] == "new" and list(loaded["alpha"]) == ["y", "b", "added"]
d = json.loads('{"c": 10}')
d["y"] = 1
assert list(d) == ["c", "y"]
d = json.loads('{"a": 1, "b": 2, "a": 3}')
assert list(d) == ["a", "b"] and d["a"] == 3
assert json.dumps(json.loads('{"b": 1, "a": 2}')) == '{"b": 1, "a": 2}'
assert json.loads(json.dumps({"z": 1, "a": {"y": 1, "b": 2}})) == {"z": 1, "a": {"y": 1, "b": 2}}
for bad in ['{"a": 1', '{"a" 1}', '[1,', '', '{1: 2}']:
    try:
        json.loads(bad)
        assert False, "expected ValueError for " + repr(bad)
    except ValueError:
        pass

# --- **kwargs arrives in sorted name order (call-site order is not kept) ---
def names(**kw):
    return list(kw)

assert names(b=1, a=2, c=3) == ["a", "b", "c"]
assert names() == [] and names(z=1) == ["z"]
assert names(**{"y": 1, "b": 2}) == ["b", "y"]

def mixed(x, **kw):
    return (x, list(kw.items()))
assert mixed(1, q=1, a=2) == (1, [("a", 2), ("q", 1)])

class K:
    def m(self, **kw):
        return list(kw)
assert K().m(zz=1, aa=2) == ["aa", "zz"]
assert (lambda **kw: list(kw))(b=1, a=2) == ["a", "b"]

def forward(**kw):
    return names(**kw)
assert forward(d=1, c=2, b=3) == ["b", "c", "d"]

kw_dict = None
def grab(**kw):
    global kw_dict
    kw_dict = kw
grab(m=1, k=2)
kw_dict["a"] = 0
assert list(kw_dict) == ["k", "m", "a"]

# --- deleting from either end and the middle, at scale ---
n = 5000
big = {i: i for i in range(n)}
for i in range(n - 1, n // 2, -1):
    del big[i]
assert len(big) == n // 2 + 1 and list(big)[-1] == n // 2
for i in range(0, n // 4):
    del big[i]
assert list(big)[0] == n // 4
while len(big) > 10:
    big.popitem()
assert list(big) == list(range(n // 4, n // 4 + 10))
big[0] = "re-added"
assert list(big)[-1] == 0 and len(big) == 11

# FIFO eviction (LRU pattern): oldest key goes first, new keys append
cache = {}
for i in range(300):
    cache[i] = i
    if len(cache) > 50:
        del cache[next(iter(cache))]
assert list(cache) == list(range(250, 300))

# --- a self-checking random model: dict vs a plain list of keys ---
import random
random.seed(7)
for trial in range(30):
    model_keys = []
    model_vals = {}
    dd = {}
    for step in range(400):
        op = random.randint(0, 9)
        k = random.randint(0, 40)
        if op <= 3:
            if k not in model_vals:
                model_keys.append(k)
            model_vals[k] = step
            dd[k] = step
        elif op == 4 and k in model_vals:
            model_keys.remove(k)
            del model_vals[k]
            del dd[k]
        elif op == 5 and k in model_vals:
            assert dd.pop(k) == model_vals[k]
            model_keys.remove(k)
            del model_vals[k]
        elif op == 6 and model_keys:
            last = model_keys.pop()
            assert dd.popitem() == (last, model_vals.pop(last))
        elif op == 7 and model_keys:
            first = model_keys[0]
            del dd[next(iter(dd))]
            model_keys.pop(0)
            del model_vals[first]
        elif op == 8:
            dd.setdefault(k, step)
            if k not in model_vals:
                model_keys.append(k)
                model_vals[k] = step
        elif op == 9 and step % 7 == 0:
            dd = dict(dd)
        assert list(dd) == model_keys, (trial, step)
        assert list(dd.values()) == [model_vals[x] for x in model_keys], (trial, step)
        assert len(dd) == len(model_keys)

# --- pop on a missing key / empty container raises the catchable Python error ---
pd = {"b": 1, 1: "x", (1, 2): "y"}
for missing in ("a", 2, (3, 4), None):
    try:
        pd.pop(missing)
        assert False, "expected KeyError"
    except KeyError as e:
        assert str(e) == repr(missing)
assert pd.pop("b") == 1 and pd.pop("zz", "dflt") == "dflt" and pd.pop("zz", None) is None
assert list(pd) == [1, (1, 2)]
try:
    {}.popitem()
    assert False, "expected KeyError"
except KeyError:
    pass
try:
    [].pop()
    assert False, "expected IndexError"
except IndexError:
    pass
try:
    [1, 2].pop(5)
    assert False, "expected IndexError"
except IndexError:
    pass
try:
    Counter().pop("nope")
    assert False, "expected KeyError"
except KeyError:
    pass
