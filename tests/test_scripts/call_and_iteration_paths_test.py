# Keyword calls on lambdas, methods and constructors, callbacks from
# builtins, and dict iteration. Everything here behaves the same under
# CPython 3.

def expect(exc, fn):
    try:
        fn()
    except exc:
        return True
    except Exception as e:
        return "wrong: " + type(e).__name__
    return "no error"

# --- lambdas with keywords ---
lam = lambda x, y=2, *rest, z=5, **extra: (x, y, rest, z, sorted(extra.items()))
assert lam(1) == (1, 2, (), 5, [])
assert lam(1, y=3) == (1, 3, (), 5, [])
assert lam(1, 2, 3, z=9, q=1) == (1, 2, (3,), 9, [("q", 1)])
assert lam(x=7) == (7, 2, (), 5, [])
assert expect(TypeError, lambda: (lambda a: a)(b=1)) == True
assert expect(TypeError, lambda: (lambda a: a)(1, a=2)) == True

# --- methods with keywords ---
class Shape:
    kind = "shape"
    def __init__(self, name, sides=0, *, colour="red"):
        self.name = name
        self.sides = sides
        self.colour = colour
    def describe(self, prefix="", upper=False):
        text = prefix + self.name
        return text.upper() if upper else text
    def scaled(self, factor=1, **opts):
        return (self.sides * factor, sorted(opts.items()))
    @staticmethod
    def make(name, sides=3):
        return Shape(name, sides=sides)
    @classmethod
    def named(cls, name="anon", sides=1):
        return cls(name, sides=sides)

s = Shape("tri", sides=3, colour="blue")
assert (s.name, s.sides, s.colour) == ("tri", 3, "blue")
assert s.describe(upper=True) == "TRI"
assert s.describe(prefix=">", upper=False) == ">tri"
assert s.scaled(factor=2, a=1, b=2) == (6, [("a", 1), ("b", 2)])
assert Shape.make("sq", sides=4).sides == 4
assert s.make(name="x").sides == 3
assert Shape.named(sides=9).sides == 9
assert s.named(name="n").name == "n"
assert expect(TypeError, lambda: s.describe(nope=1)) == True
assert expect(TypeError, lambda: s.describe(">", prefix="<")) == True
assert expect(TypeError, lambda: Shape("a", colour="x", shade=1)) == True
assert expect(TypeError, lambda: Shape(sides=2)) == True

# an instance attribute that holds a function shadows the method
s.describe = lambda prefix="", upper=False: "shadow" + prefix
assert s.describe(prefix="!") == "shadow!"
del s.describe
assert s.describe(prefix="-") == "-tri"

# inherited and missing __init__
class Child(Shape):
    pass
c = Child("kid", sides=5)
assert (c.name, c.sides) == ("kid", 5)
class Bare:
    pass
assert expect(TypeError, lambda: Bare(1)) == True
assert expect(TypeError, lambda: Bare(x=1)) == True
assert isinstance(Bare(), Bare)
class BareChild(Bare):
    pass
assert expect(TypeError, lambda: BareChild(1)) == True
class MyError(Exception):
    pass
assert MyError("boom", 2).args == ("boom", 2)
class Kw:
    def __init__(self, **kw):
        self.kw = kw
assert Kw(a=1, b=2).kw == {"a": 1, "b": 2}
class Deep(Child):
    def __init__(self, name, extra=None, **kw):
        super().__init__(name, **kw)
        self.extra = extra
d = Deep("deep", extra=1, sides=7, colour="green")
assert (d.name, d.extra, d.sides, d.colour) == ("deep", 1, 7, "green")

# --- callbacks from builtins get their own arguments ---
saved = []
def keep(*args):
    saved.append(args)
    return args[0]
assert list(map(keep, [1, 2, 3])) == [1, 2, 3]
assert saved == [(1,), (2,), (3,)]
saved.clear()
assert list(map(keep, [1, 2], [10, 20])) == [1, 2]
assert saved == [(1, 10), (2, 20)]
saved.clear()
assert list(filter(keep, [0, 1, 2])) == [1, 2]
assert saved == [(0,), (1,), (2,)]
saved.clear()
assert sorted([3, 1, 2], key=keep) == [1, 2, 3]
assert sorted(saved) == [(1,), (2,), (3,)]
saved.clear()
items = [3, 1, 2]
items.sort(key=keep)
assert items == [1, 2, 3] and sorted(saved) == [(1,), (2,), (3,)]
class Holder:
    def __init__(self, *a):
        self.a = a
held = list(map(Holder, [1, 2, 3]))
assert [h.a for h in held] == [(1,), (2,), (3,)]
def gen_one(x):
    yield x
gens = list(map(gen_one, [1, 2, 3]))
assert [list(g) for g in gens] == [[1], [2], [3]]
assert sorted(["bb", "a", "ccc"], key=len) == ["a", "bb", "ccc"]
assert list(map(lambda v: v * 2, (1, 2, 3))) == [2, 4, 6]
assert list(filter(None, [0, 1, "", "a"])) == [1, "a"]
assert max([1, 5, 3], key=lambda v: -v) == 1 and min(["bb", "a"], key=len) == "a"
assert expect(ValueError, lambda: list(map(lambda v: int("x"), [1]))) == True

# --- dict iteration ---
d = {"a": 1, "b": 2, "c": 3}
seen = []
for k in d:
    d[k] = d[k] * 10          # changing values while iterating is allowed
    seen.append(k)
assert seen == ["a", "b", "c"] and d == {"a": 10, "b": 20, "c": 30}
vals = []
for v in d.values():
    vals.append(v)
assert vals == [10, 20, 30]
pairs = []
for k, v in d.items():
    d[k] = v + 1
    pairs.append((k, v))
assert pairs == [("a", 10), ("b", 20), ("c", 30)] and d["c"] == 31
for k in list(d.keys()):
    if k != "b":
        del d[k]
assert list(d) == ["b"]
big = {}
for i in range(50):
    big[i] = i
assert list(big.keys()) == list(range(50))
assert list(big.values())[-3:] == [47, 48, 49]
assert sum(v for v in big.values()) == sum(range(50))
it = iter(big)
assert next(it) == 0 and next(it) == 1
ks = big.keys()
big[100] = 100               # a view reflects later additions
assert len(ks) == 51 and list(ks)[-1] == 100
assert [k for k in {}] == [] and list({}.items()) == []

# --- comprehensions and closures ---
fns = [lambda n=n: n * 2 for n in range(3)]
assert [f() for f in fns] == [0, 2, 4]
def counter():
    count = 0
    def inc():
        nonlocal count
        count += 1
        return count
    return inc
inc = counter()
assert [inc(), inc(), inc()] == [1, 2, 3]
assert {k: [x for x in range(k)] for k in range(3)} == {0: [], 1: [0], 2: [0, 1]}
assert sum(x * y for x in range(3) for y in range(3)) == 9
print("OK")
