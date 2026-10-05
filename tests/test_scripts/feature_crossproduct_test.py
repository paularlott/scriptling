# Cross-product of the features added in 0.29.x: sentinel x frozenset x
# copy x chains x number methods x posonly, exercising them together.

import copy

MISSING = sentinel("MISSING")
OTHER = sentinel("MISSING")

# Sentinels in frozensets: distinct even with equal names (identity keys)
group = frozenset([MISSING, OTHER])
assert len(group) == 2
assert MISSING in group and OTHER in group
assert copy.deepcopy(group) is group  # frozen: shared, not copied

# Frozenset keys survive deepcopy of a dict (immutable, shared)
d = {frozenset([1, 2]): "a", MISSING: "b"}
dc = copy.deepcopy(d)
assert dc[frozenset([2, 1])] == "a"
assert dc[MISSING] == "b"

# Chains over number methods and copies
n = 5
assert 0 < n.bit_length() < 2 ** 2 < 8
lst = [1, 2, 3]
assert 1 <= len(copy.deepcopy(lst)) <= 3

# Sentinel default in a positional-only function
def fetch(key, /, default=MISSING):
    if default is MISSING:
        return "missing"
    return default

assert fetch("k") == "missing"
assert fetch("k", "fallback") == "fallback"

def raises_type_error(fn):
    try:
        fn()
        return False
    except TypeError:
        return True

assert raises_type_error(lambda: fetch(key="k"))

# frozenset algebra with method values inside chains
fs = frozenset([1, 2])
assert fs.union([3]) == frozenset([1, 2, 3])
assert (0 < len(fs) < 5) is True

# copy.copy vs deepcopy through namedtuple fields
import collections
P = collections.namedtuple("P", ["x", "y"])
p = P([1, 2], [3, 4])
p_shallow = copy.copy(p)
assert p_shallow.x is p.x
p_deep = copy.deepcopy(p)
assert p_deep.x is not p.x and p_deep.x == [1, 2]
assert p_deep._fields == ("x", "y")

# deepcopy of an instance holding a frozenset and a sentinel
class Bundle:
    def __init__(self):
        self.tags = frozenset(["a"])
        self.marker = MISSING

b2 = copy.deepcopy(Bundle())
assert b2.tags is not None and b2.tags == frozenset(["a"])
assert b2.marker is MISSING

# set.isdisjoint over frozensets and sentinels
assert {1, 2}.isdisjoint(frozenset([3]))
assert not {MISSING}.isdisjoint({MISSING})

# abs(-0.0) is 0.0; -0.0 hashes like 0.0
assert abs(-0.0) == 0.0
assert repr(abs(-0.0)) == "0.0"

True
