# Keyword arguments to builtins, methods and library functions.
import functools
import json

# Builtins
l = [3, 1, 2]
assert sorted(l, reverse=True) == [3, 2, 1]
assert sorted(l, key=None, reverse=True) == [3, 2, 1]
assert sorted(l, reverse=False, key=lambda v: -v) == [3, 2, 1]
assert max(["bb", "a", "ccc"], key=len) == "ccc"
assert min(["bb", "a", "ccc"], key=len) == "a"
assert max([], default=5) == 5
assert list(enumerate("ab", start=1)) == [(1, "a"), (2, "b")]
assert int("ff", base=16) == 255
assert dict(a=1, b=2) == {"a": 1, "b": 2}
assert dict({"a": 1}, b=2) == {"a": 1, "b": 2}
assert sum([1, 2, 3], start=10) == 16

# String, list and dict methods
assert "a,b,c".split(",", maxsplit=1) == ["a", "b,c"]
assert "a,b,c".split(sep=",") == ["a", "b", "c"]
assert "a,b,c".rsplit(",", maxsplit=1) == ["a,b", "c"]
assert "{x}-{y}".format(x=1, y=2) == "1-2"
assert "a\nb\n".splitlines(keepends=True) == ["a\n", "b\n"]
m = [3, 1, 2]
m.sort(reverse=True)
assert m == [3, 2, 1]
m.sort(key=lambda v: v % 3)
assert m == [3, 1, 2]
d = {"a": 1}
d.update(b=2, c=3)
assert d == {"a": 1, "b": 2, "c": 3}

# Library functions
assert json.dumps({"b": 1, "a": 2}, sort_keys=True) == '{"a": 2, "b": 1}'
assert json.loads(json.dumps([1, 2], indent=None)) == [1, 2]


# functools.partial keeps its keywords across many calls with different
# call-time keywords.
def f(a, b=1, c=2):
    return a * 100 + b * 10 + c


p = functools.partial(f, c=5)
for i in range(50):
    assert p(i) == i * 100 + 10 + 5
    assert p(i, b=i % 7) == i * 100 + (i % 7) * 10 + 5
    assert p(i, c=i % 3) == i * 100 + 10 + (i % 3)
    assert p(a=i, b=2) == i * 100 + 25
q = functools.partial(sorted, reverse=True)
for i in range(20):
    assert q([i, i + 1, i - 1]) == [i + 1, i, i - 1]
    assert q([i, i + 1], reverse=False) == [i, i + 1]
r = functools.partial(int, base=2)
assert r("101") == 5
assert r("101", base=8) == 65

# ** unpacking
opts = {"reverse": True}
assert sorted(l, **opts) == [3, 2, 1]
assert sorted(l, key=None, **opts) == [3, 2, 1]
assert "a,b,c".split(",", **{"maxsplit": 1}) == ["a", "b,c"]
assert dict(**{"a": 1}, b=2) == {"a": 1, "b": 2}


def raises(exc, fn):
    try:
        fn()
    except exc:
        return True
    return False


# A keyword given twice.
assert raises(TypeError, lambda: sorted(l, reverse=True, **{"reverse": False}))
assert raises(TypeError, lambda: f(1, b=2, **{"b": 3}))

# Unknown keywords.
assert raises(TypeError, lambda: int("1", nope=1))
assert raises(TypeError, lambda: "a,b".split(",", nope=1))
assert raises(TypeError, lambda: f(1, nope=1))
assert raises(TypeError, lambda: p(1, nope=1))

# Keyword values that are themselves calls with keywords: the inner call must
# not disturb the outer call's keywords.
assert sorted(sorted(l, reverse=True), reverse=sorted([0, 1], reverse=True)[0] == 1) == [3, 2, 1]
assert max(sorted(l, key=lambda v: -v), key=lambda v: int(str(v), base=10)) == 3
assert int(str(int("11", base=2)), base=int("8", base=10)) == 3
assert "x-y-z".split("-", maxsplit=int("1", base=10)) == ["x", "y-z"]
assert json.dumps(sorted(l, reverse=True), indent=None) == "[3, 2, 1]"
assert p(int("1", base=2), b=int("10", base=2)) == 100 + 20 + 5
assert list(enumerate(sorted("cab", reverse=True), start=int("1", base=3))) == [(1, "c"), (2, "b"), (3, "a")]

print("OK")
