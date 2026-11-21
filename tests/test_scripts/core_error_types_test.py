# Core error types and results that scripts written for Python 3 depend on:
# which exception type an operation raises (so `except X` catches it), and
# results that must not be silently different. Runs unchanged under python3.
import json
import re


def raises(exc_type, fn):
    try:
        fn()
    except exc_type:
        return True
    except Exception as e:
        assert False, "expected %s, got %s: %s" % (exc_type.__name__, type(e).__name__, e)
    assert False, "expected %s, nothing raised" % exc_type.__name__


def ok(value, expected):
    assert value == expected, "got %r, expected %r" % (value, expected)
    assert type(value) == type(expected), "got %r (%s), expected %r (%s)" % (
        value, type(value).__name__, expected, type(expected).__name__)


# --- 1. bool is an int --------------------------------------------------------
ok(True | False, True)
ok(True & True, True)
ok(True & False, False)
ok(True ^ True, False)
ok(False ^ True, True)
ok(True | 2, 3)
ok(True + True, 2)
ok(abs(True), 1)
ok(abs(False), 0)
ok(round(True), 1)
ok(chr(True), "\x01")
ok([1, 2].index(True), 0)
ok([0] * True, [0])
ok(b"ab" * True, b"ab")
ok((1, 2) * False, ())
ok(list(range(True, 3)), [1, 2])
ok(bytes([True]), b"\x01")
ok(divmod(True, 1), (1, 0))
ok(pow(True, 2), 1)
ok(hex(True), "0x1")
ok(True.bit_length(), 1)
ok(True.to_bytes(1, "big"), b"\x01")
ok("%x" % True, "1")
ok(True in b"\x01", True)
b = True
b |= False
ok(b, True)
b &= False
ok(b, False)

# --- 2. pow ---------------------------------------------------------------------
raises(ZeroDivisionError, lambda: pow(0, -1))
raises(ZeroDivisionError, lambda: pow(0.0, -1))
ok(pow(2, -1), 0.5)
ok(pow(2, 3), 8)
ok(pow(2, 3, 5), 3)
ok(pow(2, -1, 5), 3)
ok(pow(3, 2, -5), -1)
raises(ValueError, lambda: pow(2, 3, 0))
raises(ValueError, lambda: pow(4, -1, 6))
raises(TypeError, lambda: pow(2.0, 3, 5))
ok(pow(3, 39), 4052555153018976267)

# --- 3. range ---------------------------------------------------------------
raises(ValueError, lambda: range(1, 2, 0))
raises(TypeError, lambda: range("a"))
raises(TypeError, lambda: range(1.5))
raises(TypeError, lambda: range(1, 2.0))
ok(list(range(3, 0, -1)), [3, 2, 1])

# --- 4. dict from pairs -----------------------------------------------------
raises(TypeError, lambda: dict([1, 2]))
ok(dict(["ab", "cd"]), {"a": "b", "c": "d"})
ok(dict([(1, 2), [3, 4]]), {1: 2, 3: 4})
raises(ValueError, lambda: dict([(1, 2, 3)]))
raises(ValueError, lambda: dict(["abc"]))
raises(TypeError, lambda: dict(1))
d = {}
d.update(["ab"])
ok(d, {"a": "b"})
raises(ValueError, lambda: {}.update([(1, 2, 3)]))
raises(TypeError, lambda: {}.update([1, 2]))
d = {"a": 1}
d.update({"a": 2}, a=3)
ok(d, {"a": 3})
d = {1: 2}
d |= [(3, 4)]
ok(d, {1: 2, 3: 4})

# --- 5. unicode and lookup errors --------------------------------------------
assert issubclass(UnicodeEncodeError, ValueError)
assert issubclass(UnicodeDecodeError, ValueError)
assert issubclass(UnicodeEncodeError, UnicodeError)
assert issubclass(UnicodeError, ValueError)
assert issubclass(KeyError, LookupError)
assert issubclass(IndexError, LookupError)
raises(UnicodeEncodeError, lambda: "é".encode("ascii"))
raises(UnicodeDecodeError, lambda: b"\xff".decode("utf-8"))
raises(UnicodeDecodeError, lambda: b"\xff".decode("ascii"))
raises(ValueError, lambda: "é".encode("ascii"))
raises(ValueError, lambda: b"\xff".decode("utf-8"))
raises(LookupError, lambda: "abc".encode("nope"))
raises(LookupError, lambda: b"abc".decode("nope"))
ok(b"\xff".decode("utf-8", "ignore"), "")
ok(str(b"ab", "utf-8"), "ab")
raises(UnicodeDecodeError, lambda: str(b"\xff", "utf-8"))
raises(UnicodeEncodeError, lambda: bytes("é", "ascii"))
ok(bytes("é", "latin-1"), b"\xe9")

# --- 6. int.to_bytes / from_bytes defaults -----------------------------------
ok((255).to_bytes(), b"\xff")
ok(int.from_bytes(b"\x01"), 1)
ok(int.from_bytes(b"\x00\x01", "big"), 1)
raises(OverflowError, lambda: (256).to_bytes())

# --- 7. exception classes as values ------------------------------------------
ok(ValueError.__name__, "ValueError")
ok(str(ValueError), "<class 'ValueError'>")
ok(repr(KeyError), "<class 'KeyError'>")
ok(str(int), "<class 'int'>")
try:
    int("x")
except Exception as e:
    ok(type(e).__name__, "ValueError")
    assert type(e) is ValueError
    ok(e.__class__.__name__, "ValueError")
    assert e.__class__ is ValueError


class MyError(ValueError):
    pass


ok(MyError.__name__, "MyError")
try:
    raise MyError("boom")
except ValueError as e:
    assert type(e) is MyError
    ok(e.__class__.__name__, "MyError")
    ok(str(e), "boom")


class Plain:
    pass


assert type(Plain()) is Plain
assert type(Plain()) == Plain
ok(str(ValueError("a", 1)), "('a', 1)")
ok(str(ValueError("a")), "a")

# --- 8. exception classes through variables ----------------------------------
exc = re.error
try:
    re.compile("(")
    assert False
except exc:
    pass
try:
    re.compile("(")
except Exception as e:
    assert isinstance(e, re.error)
try:
    re.compile("(")
except (ValueError, re.error):
    pass
errs = (KeyError, json.JSONDecodeError)
try:
    json.loads("{")
except errs:
    pass
assert issubclass(json.JSONDecodeError, ValueError)
try:
    raise exc("custom")
except re.error as e:
    ok(str(e), "custom")


def catch_with(t):
    try:
        1 / 0
    except t:
        return "caught"


ok(catch_with(ZeroDivisionError), "caught")
ok(catch_with((KeyError, ZeroDivisionError)), "caught")
handlers = [KeyError, ZeroDivisionError]
try:
    1 / 0
except handlers[1]:
    pass
user_t = MyError
try:
    raise MyError("x")
except user_t:
    pass
try:
    try:
        1 / 0
    except KeyError:
        assert False
except ZeroDivisionError:
    pass

# --- 9. json NaN and Infinity -------------------------------------------------
vals = json.loads("[NaN, Infinity, -Infinity]")
assert vals[0] != vals[0]
ok(vals[1], float("inf"))
ok(vals[2], float("-inf"))
ok(json.loads('{"a": "NaN"}'), {"a": "NaN"})
ok(json.dumps(float("nan")), "NaN")
ok(json.dumps(float("inf")), "Infinity")
ok(json.dumps([float("-inf")]), "[-Infinity]")
raises(ValueError, lambda: json.dumps(float("nan"), allow_nan=False))
raises(ValueError, lambda: json.loads("[1,"))

# --- 10. float literals out of range -------------------------------------------
ok(1e400, float("inf"))
ok(-1e400, float("-inf"))
ok(+3, 3)
ok(-(+2.5), -2.5)
raises(TypeError, lambda: +"a")

# --- 11. int/float from bytes, float edge cases --------------------------------
ok(int(b"12"), 12)
raises(ValueError, lambda: int(b"x"))
ok(float(b"1.5"), 1.5)
raises(OverflowError, lambda: int(float("inf")))
raises(ValueError, lambda: int(float("nan")))
raises(OverflowError, lambda: round(float("inf")))
raises(ValueError, lambda: round(float("nan")))
ok(round(1.5, None), 2)
ok(int(-1.9), -1)

# --- 12. formatting and list errors ------------------------------------------
raises(TypeError, lambda: "%d" % "x")
raises(TypeError, lambda: "%x" % 1.5)
raises(IndexError, lambda: "{} {}".format(1))
raises(ValueError, lambda: "{".format(1))
raises(TypeError, lambda: sorted([1, "a"]))
raises(ValueError, lambda: [1, 2, 3].remove(9))
raises(ValueError, lambda: "abc".partition(""))
raises(ValueError, lambda: "%" % ())

# --- 13. argument checks ----------------------------------------------------------
raises(TypeError, lambda: max(3, 1, 2, default=0))
raises(TypeError, lambda: min(1))
raises(TypeError, lambda: max())
ok(max([], default=5), 5)
raises(TypeError, lambda: "a".encode("utf-8", encoding="ascii"))
raises(TypeError, lambda: b"a".decode("utf-8", encoding="ascii"))
raises(TypeError, lambda: 1 & 1.0)
raises(TypeError, lambda: dict(**{1: 2}))
raises(TypeError, lambda: [1].sort(key=1))
raises(TypeError, lambda: map(abs))
raises(TypeError, lambda: getattr(1))
raises(TypeError, lambda: round(1, 2, 3))

# --- 14. walrus in comprehensions ---------------------------------------------
xs = [1, 2, 3]
r = [(y := x * 2) for x in xs]
ok(y, 6)
s_ = {(s_last := x) for x in xs}
ok(s_last, 3)
d_ = {x: (d_last := x) for x in xs}
ok(d_last, 3)
list((g_last := x) for x in xs)
ok(g_last, 3)
if any((first_big := x) > 1 for x in xs):
    ok(first_big, 2)


def running_total(values):
    total = 0
    sums = [(total := total + v) for v in values]
    return total, sums


ok(running_total([1, 2, 3]), (6, [1, 3, 6]))

# --- Problem 1 spread: typed errors from the evaluator ------------------------
raises(AttributeError, lambda: setattr(1, "x", 2))
raises(AttributeError, lambda: setattr("a", "x", 1))


def set_attr_on_int():
    x = 1
    x.y = 2


raises(AttributeError, set_attr_on_int)


def set_attr_on_list():
    x = []
    x.append = 2


raises(AttributeError, set_attr_on_list)


def set_attr_on_type():
    int.x = 1


raises(TypeError, set_attr_on_type)


def del_str_index():
    s = "ab"
    del s[0]


raises(TypeError, del_str_index)


def del_tuple_index():
    t = (1,)
    del t[0]


raises(TypeError, del_tuple_index)


def raise_non_exception():
    raise 5


raises(TypeError, raise_non_exception)


def bare_raise():
    raise


raises(RuntimeError, bare_raise)


def class_bad_base():
    class A(1):
        pass


raises(TypeError, class_bad_base)


class NoSetitem:
    pass


def setitem_missing():
    a = NoSetitem()
    a[0] = 1


raises(TypeError, setitem_missing)


class BadLen:
    def __len__(self):
        return "a"


class NegLen:
    def __len__(self):
        return -1


class BadStr:
    def __str__(self):
        return 1


class BadBool:
    def __bool__(self):
        return 1


raises(TypeError, lambda: len(BadLen()))
raises(ValueError, lambda: len(NegLen()))
raises(TypeError, lambda: str(BadStr()))
raises(TypeError, lambda: bool(BadBool()))
fn_value = lambda: 1
key_a = "a"
raises(TypeError, lambda: fn_value[0])
raises(TypeError, lambda: len[0])
raises(TypeError, lambda: [1, 2][key_a])
raises(TypeError, lambda: (1, 2)[key_a])
raises(TypeError, lambda: "ab"[key_a])
raises(TypeError, lambda: b"ab"[key_a])
raises(IndexError, lambda: [1, 2][5])
raises(KeyError, lambda: {1: 2}[3])
raises(ValueError, lambda: [1, 2][::0])
raises(ValueError, lambda: "abc"[::0])
raises(ValueError, lambda: bytes(-1))
raises(ValueError, lambda: bytes.fromhex("zz"))
raises(TypeError, lambda: enumerate())
raises(TypeError, lambda: format())
raises(TypeError, lambda: print("a", sep=1))
raises(TypeError, lambda: "abc".strip(1, 2))
raises(ValueError, lambda: str.maketrans("ab", "c"))
raises(ValueError, lambda: "{!z}".format(1))
raises(ValueError, lambda: "}".format(1))
raises(ValueError, lambda: -2 in b"ab")


def gen_stop():
    raise StopIteration
    yield 1


raises(RuntimeError, lambda: list(gen_stop()))


def reenter():
    yield next(it)


it = reenter()
raises(ValueError, lambda: next(it))


def import_missing():
    import nonexistent_module_xyz


raises(ImportError, import_missing)
raises(ModuleNotFoundError, import_missing)


def from_import_missing():
    from json import no_such_name


raises(ImportError, from_import_missing)

# Valid neighbours: nothing above over-rejects.
ok(list(iter(b"ab")), [97, 98])
ok(list(reversed(b"ab")), [98, 97])
ok(ord(b"a"), 97)
ok(set().union(), set())
s2 = {1}
s2.update()
ok(s2, {1})
lst = [1]
lst += "ab"
ok(lst, [1, "a", "b"])
lst += (2,)
ok(lst, [1, "a", "b", 2])
ok(divmod(7, 2), (3, 1))
ok(divmod(-7, 2), (-4, 1))
ok(divmod(7.5, 2), (3.0, 1.5))
ok(round(2.675, 2), 2.67)
ok(int(" 12 "), 12)
ok("%c" % 65, "A")
ok(dict(a=1), {"a": 1})
ok("".join(["a", "b"]), "ab")

print("OK")
