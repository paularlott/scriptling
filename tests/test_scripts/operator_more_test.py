import operator

# In-place functions update mutable values in place and return them.
l = [1]
assert operator.iadd(l, [2]) is l and l == [1, 2]
assert operator.imul(l, 2) is l and l == [1, 2, 1, 2]
assert operator.iconcat([1], (2,)) == [1, 2]
d = {"a": 1}
assert operator.ior(d, {"b": 2}) is d and d == {"a": 1, "b": 2}
s = {1, 2}
assert operator.iand(s, {2, 3}) is s and s == {2}
s = {1, 2}
assert operator.isub(s, {1}) is s and s == {2}
s = {1, 2}
assert operator.ixor(s, {1, 3}) is s and s == {2, 3}

# On immutable values they return the new value.
assert operator.iadd(1, 2) == 3 and operator.iadd("a", "b") == "ab" and operator.iadd((1,), (2,)) == (1, 2)
assert operator.isub(5, 2) == 3 and operator.itruediv(1, 2) == 0.5 and operator.ifloordiv(7, 2) == 3
assert operator.imod(7, 3) == 1 and operator.ipow(2, 10) == 1024
assert operator.ilshift(1, 3) == 8 and operator.irshift(8, 1) == 4
assert operator.iand(6, 3) == 2 and operator.ior(6, 1) == 7 and operator.ixor(6, 3) == 5
assert operator.__iadd__([1], [2]) == [1, 2]
try:
    operator.iadd(1, "a")
    assert False
except TypeError:
    pass

# length_hint
assert operator.length_hint([1, 2, 3]) == 3 and operator.length_hint("abc") == 3
assert operator.length_hint({"a": 1}) == 1 and operator.length_hint(range(4)) == 4
assert operator.length_hint(5, 3) == 3 and operator.length_hint(5) == 0
assert operator.length_hint({"a": 1}.keys()) == 1

# The rest of the functional forms
assert operator.truth([]) is False and operator.truth(3) is True
assert operator.is_(None, None) and operator.is_not(1, None)
assert operator.concat([1], [2]) == [1, 2] and operator.concat("a", "b") == "ab"
assert operator.contains([1, 2], 2) and not operator.contains("abc", "z")
assert operator.countOf([1, 1, 2], 1) == 2 and operator.indexOf([1, 2], 2) == 1
assert operator.index(5) == 5 and operator.index(True) == 1
assert operator.call(max, 1, 5) == 5
assert operator.call(sorted, [3, 1], reverse=True) == [3, 1]

print("OK")
