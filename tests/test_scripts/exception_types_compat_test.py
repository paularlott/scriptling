# Built-in operations raise the exception type Python raises, with the
# args and str() scripts rely on.
import os


def raises(fn):
    """Run fn and return the exception it raised, or None."""
    try:
        fn()
    except Exception as e:
        return e
    return None


def type_name(fn):
    e = raises(fn)
    if e is None:
        return None
    return type(e).__name__


# 1. KeyError: args hold the key, str() is its repr.
e = KeyError("k")
assert str(e) == "'k'"
assert e.args == ("k",)
assert repr(e) == "KeyError('k')"
assert str(KeyError()) == ""
assert str(KeyError(5)) == "5"

e = raises(lambda: {}["k"])
assert type(e).__name__ == "KeyError"
assert e.args == ("k",)
assert e.args[0] == "k"
assert str(e) == "'k'"
assert f"missing {e}" == "missing 'k'"

e = raises(lambda: {}[5])
assert e.args == (5,) and str(e) == "5"
e = raises(lambda: {}[(1, 2)])
assert e.args == ((1, 2),) and str(e) == "(1, 2)"
e = raises(lambda: {}[None])
assert e.args == (None,) and str(e) == "None"

e = raises(lambda: {}.pop("k"))
assert e.args == ("k",) and str(e) == "'k'"
e = raises(lambda: set().remove("k"))
assert e.args == ("k",) and str(e) == "'k'"
e = raises(lambda: {}.popitem())
assert type(e).__name__ == "KeyError"
assert e.args == ("popitem(): dictionary is empty",)
e = raises(lambda: "{a}".format())
assert type(e).__name__ == "KeyError" and e.args == ("a",)
e = raises(lambda: "{0} {1}".format(1))
assert type(e).__name__ == "IndexError"
e = raises(lambda: os.environ["NO_SUCH_VARIABLE_FOR_THIS_TEST"])
assert e.args == ("NO_SUCH_VARIABLE_FOR_THIS_TEST",)


def del_missing():
    d = {}
    del d["k"]


e = raises(del_missing)
assert type(e).__name__ == "KeyError" and e.args == ("k",) and str(e) == "'k'"

try:
    {"a": 1}["b"]
except LookupError as err:
    assert err.args[0] == "b"

try:
    raise KeyError("custom")
except KeyError as err:
    assert err.args[0] == "custom"
    assert str(err) == "'custom'"

# 2. Unsupported operand types are TypeError.
unsupported = [
    lambda: "a" - "b",
    lambda: "a" * "b",
    lambda: [1] - [2],
    lambda: {} + {},
    lambda: None + 1,
    lambda: "a" + 1,
    lambda: 1 + "a",
    lambda: "a" < 1,
    lambda: [1] < 1,
    lambda: None < None,
    lambda: {} < {},
    lambda: -"a",
    lambda: ~"a",
    lambda: "a" | "b",
    lambda: 1 & 1.0,
    lambda: 1.0 << 1,
    lambda: b"a" - b"b",
    lambda: "a" ** 2,
    lambda: 1.5 * "a",
    lambda: (1,) - (1,),
    lambda: {1} + {2},
    lambda: "a" // "b",
]
for i, fn in enumerate(unsupported):
    assert type_name(fn) == "TypeError", (i, type_name(fn))

e = raises(lambda: "a" - "b")
assert str(e) == "unsupported operand type(s) for -: 'str' and 'str'"
e = raises(lambda: "a" < 1)
assert str(e) == "'<' not supported between instances of 'str' and 'int'"
e = raises(lambda: "a" + 1)
assert str(e) == 'can only concatenate str (not "int") to str'
e = raises(lambda: "a" * "b")
assert str(e) == "can't multiply sequence by non-int of type 'str'"


def aug_sub():
    s = "a"
    s -= "x"


e = raises(aug_sub)
assert type(e).__name__ == "TypeError"
assert str(e) == "unsupported operand type(s) for -=: 'str' and 'str'"


def aug_list():
    items = [1]
    items -= [1]


assert type_name(aug_list) == "TypeError"


# 3. Negative shift counts are ValueError, including augmented forms.
def lshift_aug():
    x = 1
    x <<= -1


def rshift_aug():
    x = 8
    x >>= -2


def subscript_shift_aug():
    items = [1]
    items[0] <<= -1


for fn in [lambda: 1 << -1, lambda: 1 >> -1, lshift_aug, rshift_aug, subscript_shift_aug]:
    e = raises(fn)
    assert type(e).__name__ == "ValueError"
    assert str(e) == "negative shift count"

try:
    n = -1
    x = 1 << n
except ValueError as err:
    assert str(err) == "negative shift count"

# 4. Booleans are integers for indexing and slicing.
assert [10, 20][True] == 20
assert "ab"[False] == "a"
assert (1, 2)[True] == 2
assert b"ab"[True] == 98
assert range(5)[True] == 1
assert [1, 2, 3][True:] == [2, 3]
assert "abc"[False:True] == "a"
assert (1, 2, 3)[::True] == (1, 2, 3)
assert [1, 2, 3][None:2] == [1, 2]
lst = [1, 2, 3]
lst[True] = 9
assert lst == [1, 9, 3]
del lst[True]
assert lst == [1, 3]
assert type_name(lambda: [1, 2]["a":]) == "TypeError"
assert type_name(lambda: [1][::0]) == "ValueError"


# 7. Reading a local before assignment is UnboundLocalError, a NameError.
def read_before_assign():
    print(value)
    value = 1


e = raises(read_before_assign)
assert type(e).__name__ == "UnboundLocalError"
assert isinstance(e, NameError)
assert issubclass(UnboundLocalError, NameError)
try:
    read_before_assign()
except NameError:
    pass
try:
    read_before_assign()
except UnboundLocalError:
    pass
assert type_name(lambda: undefined_name_for_this_test) == "NameError"

# 8. Other built-in operations.
cases = [
    ("TypeError", lambda: [1]["a"]),
    ("TypeError", lambda: (lambda f: f())(5)),
    ("AttributeError", lambda: None.x),
    ("ValueError", lambda: int("x")),
    ("ValueError", lambda: float("x")),
    ("IndexError", lambda: [].pop()),
    ("KeyError", lambda: {}.popitem()),
    ("ZeroDivisionError", lambda: 1 / 0),
    ("ZeroDivisionError", lambda: 1 // 0),
    ("ZeroDivisionError", lambda: 1 % 0),
    ("ZeroDivisionError", lambda: 1.0 / 0),
    ("ZeroDivisionError", lambda: 1.0 // 0),
    ("ZeroDivisionError", lambda: 1.0 % 0),
    ("ZeroDivisionError", lambda: divmod(1, 0)),
    ("ZeroDivisionError", lambda: divmod(1.0, 0)),
    ("ZeroDivisionError", lambda: 0 ** -1),
    ("ZeroDivisionError", lambda: 0.0 ** -1),
    ("TypeError", lambda: len(5)),
    ("TypeError", lambda: {[1]: 2}),
    ("TypeError", lambda: {[1]}),
    ("TypeError", lambda: [x for x in 5]),
    ("TypeError", lambda: list(5)),
    ("IndexError", lambda: [1][5]),
    ("IndexError", lambda: "a"[5]),
    ("IndexError", lambda: (1,)[5]),
    ("ValueError", lambda: [1].index(2)),
    ("ValueError", lambda: (1,).index(2)),
    ("ValueError", lambda: "a".index("b")),
    ("ValueError", lambda: chr(-1)),
    ("TypeError", lambda: ord("ab")),
    ("TypeError", lambda: int(None)),
    ("TypeError", lambda: abs("a")),
    ("ValueError", lambda: max([])),
    ("ValueError", lambda: min([])),
    ("TypeError", lambda: 1 in 5),
    ("TypeError", lambda: sorted([1, "a"])),
    ("ValueError", lambda: [1].remove(2)),
    ("TypeError", lambda: "a" * 1.5),
    ("AttributeError", lambda: (5).foo),
    ("AttributeError", lambda: getattr(5, "foo")),
    ("TypeError", lambda: {}[[1]]),
    ("TypeError", lambda: range("a")),
    ("ValueError", lambda: int("1.5")),
    ("StopIteration", lambda: next(iter([]))),
    ("TypeError", lambda: sum(["a"])),
    ("ValueError", lambda: "a".split("")),
    ("TypeError", lambda: b"a" + "a"),
    ("TypeError", lambda: "%s %s" % ("a",)),
    ("ValueError", lambda: int("z", 10)),
    ("TypeError", lambda: isinstance(1, 5)),
    ("TypeError", lambda: [1, 2] * "a"),
    ("TypeError", lambda: [1][1.0]),
    ("AttributeError", lambda: "a".nope()),
    ("AttributeError", lambda: [].nope()),
    ("ValueError", lambda: int("")),
    ("TypeError", lambda: tuple(5)),
    ("IndexError", lambda: "a"[-5]),
    ("ValueError", lambda: bytes([300])),
]
for expected, fn in cases:
    got = type_name(fn)
    assert got == expected, (expected, got)


def unpack_too_many():
    a, b = [1, 2, 3]


def unpack_too_few():
    a, b, c = [1, 2]


def unpack_int():
    a, b = 5


def assign_out_of_range():
    items = [1]
    items[5] = 2


def assign_tuple():
    t = (1,)
    t[0] = 2


def del_out_of_range():
    items = []
    del items[0]


for expected, fn in [
    ("ValueError", unpack_too_many),
    ("ValueError", unpack_too_few),
    ("TypeError", unpack_int),
    ("IndexError", assign_out_of_range),
    ("TypeError", assign_tuple),
    ("IndexError", del_out_of_range),
]:
    got = type_name(fn)
    assert got == expected, (expected, got)

e = raises(del_out_of_range)
assert str(e) == "list assignment index out of range"
e = raises(lambda: divmod(1, 0))
assert e.args == ("division by zero",) or e.args == ("integer division or modulo by zero",)

print("OK")
