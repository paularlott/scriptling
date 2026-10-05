# Test the copy library: copy.copy (shallow) and copy.deepcopy

_builtin_copy = copy  # captured before the import shadows the builtin
import copy

# Shallow copy: new container, shared elements
original = {"items": [1, 2], "name": "config"}
shallow = copy.copy(original)
assert shallow == original
assert shallow is not original
shallow["name"] = "changed"
assert original["name"] == "config"
inner = original["items"]
shallow["items"].append(3)
assert original["items"] == [1, 2, 3]  # shared nested list

# The copy() builtin and copy.copy agree
lst = [1, [2]]
by_builtin = _builtin_copy(lst)
by_library = copy.copy(lst)
assert by_builtin == by_library
assert by_builtin[1] is lst[1]

# Deep copy: nested objects are independent
src = {"items": [1, 2], "meta": {"on": True}}
deep = copy.deepcopy(src)
deep["items"].append(3)
deep["meta"]["on"] = False
assert src == {"items": [1, 2], "meta": {"on": True}}

deep2 = copy.deepcopy(src)
deep2["meta"]["on"] = False
assert src["meta"]["on"] is True

# Shared references stay shared in the copy
shared = [7, 8]
holder = {"a": shared, "b": shared}
holder_copy = copy.deepcopy(holder)
assert holder_copy["a"] is holder_copy["b"]
assert holder_copy["a"] is not shared

# Cycles are handled
cyc = [1]
cyc.append(cyc)
cyc_copy = copy.deepcopy(cyc)
assert cyc_copy[1] is cyc_copy
assert cyc_copy[0] == 1
assert len(cyc_copy) == 2

# All-atomic tuples are shared, like Python
t = (1, "a", 2.5, True, None)
assert copy.deepcopy(t) is t
assert copy.copy(t) is t

# Tuples with mutable elements are copied deeply
t2 = (1, [2, 3])
t2_copy = copy.deepcopy(t2)
assert t2_copy == t2
assert t2_copy[1] is not t2[1]
t2[1].append(4)
assert t2_copy[1] == [2, 3]

# Sets: mutable sets are copied, frozen sets shared
s = {1, 2}
s_copy = copy.deepcopy(s)
assert s_copy == s
s_copy.add(3)
assert 3 not in s
fs = frozenset([1, 2])
assert copy.deepcopy(fs) is fs

# Instances: fields are deep-copied without re-running __init__
class Point:
    def __init__(self):
        self.history = [0]
        self.label = "origin"

p = Point()
p.history.append(1)
p2 = copy.deepcopy(p)
assert isinstance(p2, Point)
assert p2.history == [0, 1]
assert p2.history is not p.history
p2.history.append(2)
assert p.history == [0, 1]

# Instance cycles
class Node:
    def __init__(self):
        self.peer = None

a = Node()
b = Node()
a.peer = b
b.peer = a
a_copy = copy.deepcopy(a)
assert a_copy.peer.peer is a_copy
assert a_copy.peer is not b

# __deepcopy__(self, memo) controls copying, as in Python
class Token:
    def __init__(self, value):
        self.value = value

    def __deepcopy__(self, memo):
        return Token(self.value + "-copied")

tok = Token("x")
tok_copy = copy.deepcopy(tok)
assert isinstance(tok_copy, Token)
assert tok_copy.value == "x-copied"
assert tok.value == "x"

# __deepcopy__ with the one-argument signature also works
class Simple:
    def __init__(self):
        self.n = 1

    def __deepcopy__(self):
        return Simple()

sm = Simple()
sm_copy = copy.deepcopy(sm)
assert isinstance(sm_copy, Simple)
assert sm_copy.n == 1
sm_copy.n = 2
assert sm.n == 1

# Scalars and functions are shared, like Python
assert copy.deepcopy(5) == 5
assert copy.deepcopy("abc") == "abc"
assert copy.deepcopy(None) is None
fn = lambda x: x
assert copy.deepcopy(fn) is fn

# Dicts with tuple keys survive a deep copy
keyed = {(1, 2): "tuple-key"}
keyed_copy = copy.deepcopy(keyed)
assert keyed_copy[(1, 2)] == "tuple-key"

# A failing __deepcopy__ propagates as the call's error instead of being
# embedded inside a corrupt copy
class Bad:
    def __deepcopy__(self, memo):
        raise ValueError("cannot copy me")

class Good:
    def __init__(self):
        self.n = 1

    def __deepcopy__(self, memo):
        return Good()

try:
    copy.deepcopy({"ok": Good(), "bad": Bad()})
    embedded_error = "no-raise"
except ValueError:
    embedded_error = "raised"

assert embedded_error == "raised"

# Deeply nested structures report a catchable error instead of crashing
deep = [0]
for i in range(100005):
    deep = [deep]
try:
    copy.deepcopy(deep)
    depth_error = "no-raise"
except Exception as e:
    depth_error = str(e)

assert "nesting depth" in depth_error

True
