import operator
from operator import itemgetter, attrgetter, methodcaller
import functools

rows = [(3, "c", 1.5), (1, "a", 2.5), (2, "b", 0.5)]

# itemgetter as a key function
assert sorted(rows, key=itemgetter(0)) == [(1, "a", 2.5), (2, "b", 0.5), (3, "c", 1.5)]
assert sorted(rows, key=itemgetter(2, 0))[0] == (2, "b", 0.5)
assert min(rows, key=itemgetter(2)) == (2, "b", 0.5)
assert max(rows, key=itemgetter(1)) == (3, "c", 1.5)
assert list(map(itemgetter(1), rows)) == ["c", "a", "b"]
assert itemgetter(1, 0)(("x", "y")) == ("y", "x")
assert itemgetter(-1)("abc") == "c"
assert itemgetter(slice(1, 3))([1, 2, 3, 4]) == [2, 3]
dicts = [{"n": "x", "a": 3}, {"n": "y", "a": 1}]
assert [d["n"] for d in sorted(dicts, key=itemgetter("a"))] == ["y", "x"]
data = [5, 3, 4]
data.sort(key=operator.neg)
assert data == [5, 4, 3]

try:
    itemgetter(5)([1])
    assert False, "expected IndexError"
except IndexError:
    pass
try:
    itemgetter("q")({})
    assert False, "expected KeyError"
except KeyError:
    pass


# attrgetter, including dotted names
class City:
    def __init__(self, name):
        self.name = name


class Person:
    def __init__(self, name, age, city):
        self.name = name
        self.age = age
        self.city = City(city)


people = [Person("bo", 30, "Oslo"), Person("al", 25, "Rome")]
assert [p.name for p in sorted(people, key=attrgetter("age"))] == ["al", "bo"]
assert attrgetter("name", "city.name")(people[0]) == ("bo", "Oslo")
assert attrgetter("city.name")(people[1]) == "Rome"
try:
    attrgetter("missing")(people[0])
    assert False, "expected AttributeError"
except AttributeError:
    pass

# methodcaller
assert list(map(methodcaller("upper"), ["a", "b"])) == ["A", "B"]
assert methodcaller("split", ",")("a,b") == ["a", "b"]
assert methodcaller("split", sep="-", maxsplit=1)("a-b-c") == ["a", "b-c"]

# with reduce
assert functools.reduce(operator.add, [1, 2, 3, 4]) == 10
assert functools.reduce(operator.mul, range(1, 6)) == 120
assert functools.reduce(operator.concat, [[1], [2]], []) == [1, 2]

# function forms
assert [operator.add(1, 2), operator.sub(5, 3), operator.mul(2, 3)] == [3, 2, 6]
assert [operator.truediv(7, 2), operator.floordiv(7, 2), operator.mod(7, 3), operator.pow(2, 10)] == [3.5, 3, 1, 1024]
assert [operator.neg(3), operator.pos(-3), operator.abs(-3), operator.invert(5)] == [-3, -3, 3, -6]
assert operator.not_(0) is True and operator.truth([]) is False and operator.truth([1]) is True
assert [operator.eq(1, 1), operator.ne(1, 2), operator.lt(1, 2), operator.le(2, 2), operator.gt(1, 2), operator.ge(3, 2)] == [True, True, True, True, False, True]
assert [operator.and_(6, 3), operator.or_(6, 3), operator.xor(6, 3), operator.lshift(1, 4), operator.rshift(16, 2)] == [2, 7, 5, 16, 4]
assert operator.contains([1, 2], 2) and not operator.contains("abc", "z")
assert operator.concat("ab", "cd") == "abcd"
assert operator.index(7) == 7
assert operator.is_(None, None) and operator.is_not(1, None)
assert operator.countOf([1, 2, 1], 1) == 2 and operator.indexOf([4, 5], 5) == 1
assert operator.__add__(2, 3) == 5
assert list(map(operator.mul, [1, 2], [3, 4])) == [3, 8]

d = {"a": 1}
operator.setitem(d, "b", 2)
operator.delitem(d, "a")
assert d == {"b": 2} and operator.getitem(d, "b") == 2

try:
    operator.truediv(1, 0)
    assert False, "expected ZeroDivisionError"
except ZeroDivisionError:
    pass
try:
    operator.add("a", 1)
    assert False, "expected TypeError"
except TypeError:
    pass
try:
    operator.index(7.5)
    assert False, "expected TypeError"
except TypeError:
    pass


# operators dispatch to dunder methods
class V:
    def __init__(self, x):
        self.x = x

    def __add__(self, other):
        return V(self.x + other.x)

    def __lt__(self, other):
        return self.x < other.x


assert operator.add(V(1), V(2)).x == 3
assert operator.lt(V(1), V(2)) is True
assert sorted([V(3), V(1)], key=attrgetter("x"))[0].x == 1

print("OK")
