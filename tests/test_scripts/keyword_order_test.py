# Keyword arguments keep the order they were written in.
import json
from collections import OrderedDict
from functools import partial


def names(**kw):
    return list(kw)


assert list(dict(z=1, a=2, m=3)) == ["z", "a", "m"]
assert list(dict([("a", 1)], y=2, key2=1)) == ["a", "y", "key2"]
assert names(z=1, a=2, m=3) == ["z", "a", "m"]
assert names(**{"q": 1, "b": 2}) == ["q", "b"]
assert names(k=0, **{"q": 1, "b": 2}) == ["k", "q", "b"]

d = {}
d.update(z=1, a=2)
assert list(d) == ["z", "a"]


class C:
    def __init__(self, **kw):
        self.kw = kw

    def m(self, *a, **kw):
        return list(kw.items())


c = C(z=1, a=2)
assert list(c.kw) == ["z", "a"]
assert c.m(1, y=1, b=2) == [("y", 1), ("b", 2)]
assert (lambda **kw: list(kw))(z=1, a=2) == ["z", "a"]

# Keyword values are evaluated in the order written.
order = []


def note(n):
    order.append(n)
    return n


names(b=note("b"), a=note("a"))
assert order == ["b", "a"]

assert json.dumps(dict(name="x", age=3)) == '{"name": "x", "age": 3}'
assert list(OrderedDict(z=1, a=2)) == ["z", "a"]
assert "{z}{a}".format(z=1, a=2) == "12"
assert partial(names, z=1, a=2)(m=3) == ["z", "a", "m"]

print("OK")
