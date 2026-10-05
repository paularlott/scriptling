# obj.__dict__ and vars(obj): attributes in assignment order, and writes through
# the dict (update, d[k] = v, del, pop, setdefault, clear) land on the object.
import copy
import json

# --- the config-bag idiom ---
class Bag:
    def __init__(self, **kw):
        self.__dict__.update(kw)

b = Bag(a=1, b=2, c=3)
assert (b.a, b.b, b.c) == (1, 2, 3)
assert b.__dict__ == {"a": 1, "b": 2, "c": 3} and vars(b) == b.__dict__

# --- order follows assignment, including past the first few attributes ---
class Many:
    def __init__(self):
        self.z = 1
        self.a = 2
        self.m = [3]
        self.q = 4
        self.r = 5
        self.s = 6
        self.t = 7

m = Many()
assert list(m.__dict__) == ["z", "a", "m", "q", "r", "s", "t"]
m.a = 99  # re-assigning keeps the position
assert list(m.__dict__) == ["z", "a", "m", "q", "r", "s", "t"]
m.late = 1
assert list(m.__dict__)[-1] == "late"

# deleting attributes keeps the order of the rest, and new ones still append
class Del:
    def __init__(self):
        self.x = 1
        self.y = 2
        self.p = 3
        self.u = 4
        self.v = 5
        self.w = 6

d = Del()
del d.x
del d.p
d.n = 7
assert list(d.__dict__) == ["y", "u", "v", "w", "n"]

# --- writes through __dict__ reach the object ---
m.__dict__["new"] = 9
m.__dict__.update(extra=1)
assert m.new == 9 and m.extra == 1
del m.__dict__["z"]
assert not hasattr(m, "z") and list(m.__dict__)[0] == "a"
assert m.__dict__.pop("a") == 99 and not hasattr(m, "a")
assert m.__dict__.setdefault("sd", 5) == 5 and m.sd == 5
assert m.__dict__.setdefault("sd", 6) == 5 and m.sd == 5
view = m.__dict__
view.clear()
assert vars(m) == {} and not hasattr(m, "m")

# vars(obj) is the same view
class V:
    def __init__(self):
        self.k = 1

v = V()
vars(v)["added"] = 2
assert v.added == 2 and list(v.__dict__) == ["k", "added"]

# a __dict__ write that shadows a method takes effect
class T:
    def method(self):
        return "method"

t = T()
assert t.method() == "method"
t.__dict__["method"] = lambda: "shadow"
assert t.method() == "shadow"

# __dict__ writes bypass property setters, as in Python
class U:
    def __init__(self):
        self._x = 1
    @property
    def x(self):
        return self._x
    @x.setter
    def x(self, value):
        self._x = value * 10

u = U()
u.x = 2
assert u._x == 20
u.__dict__["x"] = 5
assert u.__dict__ == {"_x": 20, "x": 5}

# --- serialization and iteration idioms ---
class S:
    def __init__(self):
        self.name = "n"
        self.tags = ["a"]
        self.n = 1
        self.f = 1.5
        self.ok = True
        self.none = None
    def to_dict(self):
        return dict(self.__dict__)
    def describe(self):
        return ", ".join("%s=%s" % (k, v) for k, v in self.__dict__.items())

s = S()
assert json.dumps(s.__dict__) == '{"name": "n", "tags": ["a"], "n": 1, "f": 1.5, "ok": true, "none": null}'
assert s.to_dict() == s.__dict__ and s.describe().startswith("name=n, tags=['a'], n=1")
assert [k for k in s.__dict__] == ["name", "tags", "n", "f", "ok", "none"]
assert getattr(s, "__dict__") == s.__dict__ and hasattr(s, "__dict__")

# copies of the view are plain dicts: changing them does not touch the object
snapshot = s.__dict__.copy()
snapshot["extra"] = 1
assert not hasattr(s, "extra")
as_dict = dict(s.__dict__)
as_dict["extra"] = 1
assert not hasattr(s, "extra")
deep = copy.deepcopy(s.__dict__)
deep["tags"].append("b")
assert s.tags == ["a"]

# inheritance: the view covers fields set by base and derived __init__
class Base:
    def __init__(self):
        self.base = 1

class Child(Base):
    def __init__(self):
        super().__init__()
        self.child = 2

assert list(Child().__dict__) == ["base", "child"]

# --- assigning a whole new __dict__ is not supported (a clear error) ---
try:
    u.__dict__ = {}
    assert False, "expected AttributeError"
except AttributeError:
    pass
