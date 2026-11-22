# type(x) is the type itself, as in Python.
import datetime
from collections import OrderedDict, defaultdict


class Point:
    def __init__(self, x=0):
        self.x = x

    def moved(self, dx):
        # A method building another instance of whatever class self is.
        return type(self)(self.x + dx)


class Point3(Point):
    pass


p = Point3(1).moved(2)
assert type(p) is Point3 and p.x == 3

# Builtin types are the builtins, and can be called.
assert type(1) is int and type("") is str and type([]) is list
assert type(1.5)("2.5") == 2.5 and type("a")(5) == "5" and type([1])((1, 2)) == [1, 2]
assert type(True) is bool and type(True) is not int
assert {type(1): "i"}[int] == "i"

# Types print as Python prints them.
assert str(type(1)) == "<class 'int'>" and repr(type([])) == "<class 'list'>"
assert f"{type(None)}" == "<class 'NoneType'>"
assert type(type(1)) is type and type(int) is type and isinstance(type(1), type)

# Types with no builtin name still have one, compare by identity and work
# with isinstance.
assert type(None) is type(None) and type(None).__name__ == "NoneType"
assert isinstance(None, type(None)) and not isinstance(0, type(None))
assert type(None)() is None
assert type(len).__name__ == "builtin_function_or_method"
assert type(lambda: 0).__name__ == "function"
try:
    type(len)()
    assert False, "made a builtin_function_or_method"
except TypeError:
    pass

# Library types.
assert type(defaultdict(list)) is defaultdict and type(OrderedDict()) is OrderedDict
d = datetime.date(2020, 1, 2)
assert type(d) is datetime.date and type(d)(2021, 3, 4) == datetime.date(2021, 3, 4)
assert type(datetime.datetime(2020, 1, 1)) is datetime.datetime

# Exceptions report the class they were raised as.
class AppError(Exception):
    pass


try:
    raise AppError("x")
except Exception as e:
    assert type(e) is AppError and type(e).__name__ == "AppError"
try:
    {}["k"]
except Exception as e:
    assert type(e) is KeyError

# range, map and the other lazy iterables are types.
assert type(range(2)) is range and isinstance(range(2), range)
assert isinstance(map(str, []), map) and isinstance(zip(), zip) and isinstance(enumerate([]), enumerate)
assert str(range) == "<class 'range'>" and isinstance(range, type)
