import functools

# ascii()
assert ascii("héllo") == "'h\\xe9llo'"
assert ascii(["é", 1, "☃", "\U0001F600"]) == "['\\xe9', 1, '\\u2603', '\\U0001f600']"
assert f"{'é'!a}" == "'\\xe9'"

# Ellipsis
assert str(Ellipsis) == "Ellipsis" and repr(Ellipsis) == "Ellipsis"
assert type(Ellipsis).__name__ == "ellipsis"
assert Ellipsis is Ellipsis

# iter(callable, sentinel)
assert list(iter(lambda: 1, 1)) == []
data = [1, 2, 3, None, 5]
source = iter(data)
assert list(iter(lambda: next(source), None)) == [1, 2, 3]
stack = [3, 0, 2, 1]
assert list(iter(stack.pop, 0)) == [1, 2]


# locals() and globals()
def scope(a, b=2):
    c = a + b
    return locals()


assert scope(1) == {"a": 1, "b": 2, "c": 3}
assert "scope" in globals() and "data" in globals()

# sum with a start value
assert sum([[1], [2]], []) == [1, 2]
assert sum([(1,), (2,)], ()) == (1, 2)
assert sum([1, 2], start=10) == 13
assert sum([1.5, 2], 1) == 4.5
for bad in (lambda: sum(["a"], ""), lambda: sum([1, "a"]), lambda: sum([[1], [2]])):
    try:
        bad()
        assert False, "expected TypeError"
    except TypeError:
        pass


class Vec:
    def __init__(self, x):
        self.x = x

    def __add__(self, other):
        return Vec(self.x + other.x)


assert sum([Vec(1), Vec(2)], Vec(0)).x == 3


# object
_MISSING = object()
assert _MISSING is _MISSING and object() is not object() and object() != object()


def get(d, key, default=_MISSING):
    if key in d:
        return d[key]
    if default is _MISSING:
        raise KeyError(key)
    return default


assert get({}, "a", 5) == 5


class C(object):
    def __init__(self, x):
        self.x = x


class D(C):
    pass


assert C(3).x == 3 and D(4).x == 4
assert isinstance(C(1), object) and isinstance(None, object) and isinstance(1, object)
assert isinstance(len, object) and isinstance(C, object)
assert issubclass(C, object) and issubclass(D, C) and issubclass(str, object)
assert type(object()).__name__ == "object"
assert str(object) == "<class 'object'>"


# issubclass and isinstance with builtin types
class MyError(ValueError):
    pass


assert issubclass(bool, int) and not issubclass(int, bool)
assert issubclass(int, object) and issubclass(dict, object)
assert issubclass(MyError, Exception) and issubclass(MyError, ValueError)
assert issubclass(MyError, (TypeError, ValueError))
assert issubclass(KeyError, LookupError) and issubclass(ValueError, (int, Exception))
assert issubclass(bool, (str, int)) and not issubclass(C, int)
assert isinstance("x", (str, int)) and isinstance(True, int) and not isinstance(1.0, int)
try:
    issubclass(1, int)
    assert False, "expected TypeError"
except TypeError:
    pass

# str.format field access
assert "{0[1]} {1.real}".format([1, 2], 3) == "2 3"
assert "{name[key]}".format(name={"key": 5}) == "5"
assert "{obj.x}".format(obj=C(7)) == "7"
assert "{0[0][1]}".format([[1, 2]]) == "2"
assert "{[a]}".format({"a": 9}) == "9"
assert "{0.x:>5}|".format(C(7)) == "    7|"
try:
    "{nope}".format()
    assert False, "expected KeyError"
except KeyError:
    pass
try:
    "{}".format()
    assert False, "expected IndexError"
except IndexError:
    pass
try:
    "{0.nope}".format(C(1))
    assert False, "expected AttributeError"
except AttributeError:
    pass


# __doc__ and __name__
def documented():
    """my doc"""
    return 1


def undocumented():
    return 1


class K:
    """Doc of K"""

    def m(self):
        "method doc"
        return 1


class Sub(K):
    pass


assert documented.__doc__ == "my doc" and undocumented.__doc__ is None
assert documented.__name__ == "documented" and documented.__qualname__ == "documented"
assert K.__doc__ == "Doc of K" and K().__doc__ == "Doc of K" and Sub.__doc__ is None
assert K.__name__ == "K" and K.m.__doc__ == "method doc"
assert K().m.__doc__ == "method doc" and K().m.__name__ == "m"
assert (lambda: 1).__name__ == "<lambda>"
assert (5).real == 5 and (5).imag == 0 and (2.5).real == 2.5


def multi():
    """  First line.

        Indented more
    Back.
    """


assert multi.__doc__ == "First line.\n\n    Indented more\nBack.\n"


# functools.wraps carries __name__, __doc__ and __wrapped__
def deco(fn):
    @functools.wraps(fn)
    def wrapper(*a, **k):
        """wrapper doc"""
        return fn(*a, **k)

    return wrapper


@deco
def wrapped_fn(x):
    """The doc."""
    return x


assert wrapped_fn.__name__ == "wrapped_fn" and wrapped_fn.__doc__ == "The doc."
assert wrapped_fn(4) == 4 and wrapped_fn.__wrapped__(5) == 5


class WithDeco:
    @deco
    def meth(self):
        return 1


assert WithDeco().meth() == 1 and WithDeco.meth.__name__ == "meth"


# raise ... from ...
def cause_of(fn):
    try:
        fn()
    except Exception as e:
        return e.__cause__


def chained():
    try:
        raise KeyError("a")
    except KeyError as e:
        raise ValueError("b") from e


def unchained():
    try:
        raise KeyError("a")
    except KeyError:
        raise ValueError("b") from None


def plain():
    raise ValueError("c")


assert type(cause_of(chained)).__name__ == "KeyError"
assert cause_of(unchained) is None and cause_of(plain) is None
try:
    raise RuntimeError("x") from ValueError("y")
except RuntimeError as e:
    assert str(e.__cause__) == "y"

print("OK")
