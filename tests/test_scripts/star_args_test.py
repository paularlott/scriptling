# *args is a tuple, and arguments written after a *unpack keep their order.

def collect(*args):
    return args

assert collect() == ()
assert collect(1, 2) == (1, 2)
assert isinstance(collect(1), tuple)
assert not isinstance(collect(1), list)
assert collect(*[1, 2]) == (1, 2)
assert collect()[1:] == ()
assert not collect()

# Immutable, as in Python
def mutate(*args):
    args.append(1)

try:
    mutate(1)
    assert False, "expected AttributeError"
except AttributeError:
    pass

def mutate_item(*args):
    args[0] = 9

try:
    mutate_item(1)
    assert False, "expected TypeError"
except TypeError:
    pass

# Works through lambdas, methods and generators
assert (lambda *a: a)(1, 2) == (1, 2)
assert (lambda x, *r: r)(1, 2, 3) == (2, 3)

class C:
    def m(self, *a):
        return a
    def n(self, *a, **k):
        return (a, k)

c = C()
assert c.m() == () and c.m(1, 2) == (1, 2)
assert c.n(5, k=1) == ((5,), {"k": 1})

def gen(*a):
    for x in a:
        yield x
assert list(gen(1, 2)) == [1, 2]

# Order is preserved around *unpack
def f(a, b, *rest):
    return (a, b, rest)

assert f(*[1], 2, 3, 4) == (1, 2, (3, 4))
assert f(0, *[1], 2) == (0, 1, (2,))
assert f(*[1], *[2], 3) == (1, 2, (3,))
assert f(*[], 1, 2) == (1, 2, ())
assert f(1, *[2, 3], 4, 5) == (1, 2, (3, 4, 5))
assert collect(*[1, 2], 3) == (1, 2, 3)
assert collect(*[3, 1, 2], 5) == (3, 1, 2, 5)
assert collect(1, *[2], *[3], 4, 5) == (1, 2, 3, 4, 5)
assert collect(*"ab", "c") == ("a", "b", "c")
assert collect(*(1, 2), *iter([3]), 4) == (1, 2, 3, 4)

# Methods, keywords and ** unpacking alongside
assert c.m(1, *[2, 3], 4) == (1, 2, 3, 4)
assert c.m(*[1], 2) == (1, 2)
assert c.n(*[1], 2, k=3) == ((1, 2), {"k": 3})
assert c.n(5, *[6], *[7], 8, **{"z": 1}) == ((5, 6, 7, 8), {"z": 1})

# Builtins
import os
assert os.path.join(*["a", "b"], "file.txt") == "a/b/file.txt"
assert max(*[1, 9], 5) == 9
assert "{} {} {}".format(*[1, 2], 3) == "1 2 3"

# Decorator forwarding
def deco(fn):
    def w(*a, **k):
        return fn(*a, **k)
    return w

@deco
def add(a, b, c=0):
    return a + b + c

assert add(1, 2) == 3
assert add(*[1], 2, c=3) == 6

# Arguments evaluate in source order, positionals and unpacks interleaved
order_log = []
def tag(t):
    order_log.append(t)
    return t

assert collect(tag("a"), *[tag("b")], tag("c")) == ("a", "b", "c")
assert order_log == ["a", "b", "c"]
order_log.clear()
assert collect(*[tag("x")], tag("y"), *[tag("z")], tag("w")) == ("x", "y", "z", "w")
assert order_log == ["x", "y", "z", "w"]

# Any __iter__ object can be splatted, in any position
class T:
    def __iter__(self):
        return iter([7, 8])

assert f(*T(), 9) == (7, 8, (9,))
