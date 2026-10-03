# Python 3 semantics fixed after review for 0.28: lazy map/filter/any/all over
# iterators, != via __eq__, int()/float() parsing, key= callables, Counter
# counts and format specs. Runs unchanged under CPython.
import itertools
from collections import Counter

# --- infinite iterators are consumed lazily ---
assert next(filter(lambda x: x > 5, itertools.count())) == 6
assert list(zip("ab", map(lambda x: x * 2, itertools.count()))) == [("a", 0), ("b", 2)]
assert any(map(lambda x: x > 3, itertools.count()))
assert not all(map(lambda x: x < 3, itertools.count()))
assert list(itertools.islice(enumerate(map(str, itertools.count(1))), 2)) == [(0, "1"), (1, "2")]
assert list(map(lambda a, b: a + b, [1, 2, 3], itertools.count(10))) == [11, 13, 15]
assert sum(itertools.islice(map(lambda x: x * x, itertools.count()), 5)) == 30
assert list(filter(None, iter([0, 1, "", "a"]))) == [1, "a"]
assert any(iter([0, 2])) and not any(iter([])) and all(iter([])) and not all(iter([0]))


def bad(x):
    if x == 2:
        raise ValueError("bad")
    return x


try:
    list(map(bad, iter([1, 2, 3])))
    assert False, "map should raise"
except ValueError as e:
    assert str(e) == "bad"

# --- != falls back to the inverse of __eq__ ---
class P:
    def __init__(self, x):
        self.x = x

    def __eq__(self, other):
        return isinstance(other, P) and self.x == other.x


class Q(P):
    pass


assert not (P(1) != P(1)) and P(1) != P(2) and P(1) != 3 and 3 != P(1)
assert not (Q(1) != Q(1))


class N:
    def __eq__(self, other):
        return True

    def __ne__(self, other):
        return "custom"


assert (N() != 1) == "custom"

# --- int() and float() parse strings like Python ---
for text, base, want in [("1_000", 10, 1000), (" -12 ", 10, -12), ("0x_ff", 16, 255), ("0xff", 0, 255),
                         ("0b1_0", 0, 2), ("0o17", 8, 15), ("00", 0, 0), ("0_0", 0, 0), ("zz", 36, 1295),
                         ("0xE_E", 16, 238), ("-0x1f", 16, -31)]:
    assert int(text, base) == want, text
for text, base in [("abc", 10), ("1__0", 10), ("_1", 10), ("1_", 10), ("017", 0), ("", 10),
                   ("0b", 0), ("12", 2), ("1.5", 10), ("+-1", 10), ("1 2", 10)]:
    try:
        int(text, base)
        assert False, "int(%r, %d) should raise" % (text, base)
    except ValueError as e:
        assert str(e) == "invalid literal for int() with base %d: %r" % (base, text), str(e)

for text, want in [("1.5", 1.5), (" -2.5e3 ", -2500.0), ("1_000.5", 1000.5), ("+1", 1.0), (".5", 0.5), ("5.", 5.0)]:
    assert float(text) == want, text
assert float("inf") > 1e308 and float("-Infinity") < -1e308 and float("1e999") > 1e308
for text in ["1__0", "1.5abc", "", "0x1p-2", "--1", "1_e5", "abc"]:
    try:
        float(text)
        assert False, "float(%r) should raise" % text
    except ValueError as e:
        assert str(e) == "could not convert string to float: %r" % text, str(e)

# --- key= accepts any callable ---
class Keys:
    def neg(self, x):
        return -x


class Neg:
    def __call__(self, x):
        return -x


assert sorted(["b", "A", "c"], key=str.lower) == ["A", "b", "c"]
assert sorted([1, 3, 2], key=Keys().neg) == [3, 2, 1]
assert sorted([1, 3, 2], key=Neg()) == [3, 2, 1] and max([1, 3, 2], key=Neg()) == 1
assert sorted([3, 1], key=None) == [1, 3] and min([3, -5], key=abs) == 3
assert callable(Keys) and callable(Keys().neg) and callable(Neg()) and not callable(Keys())
try:
    sorted([1], key=5)
    assert False, "key=5 should raise"
except TypeError as e:
    assert str(e) == "'int' object is not callable"

# --- Counter ---
c = Counter({"a": 2.5, "b": 1})
assert c["a"] == 2.5 and c.most_common() == [("a", 2.5), ("b", 1)] and c.total() == 3.5
assert Counter(a=1, b=0) == Counter(a=1) and Counter(a=1) == Counter(a=1.0)
assert Counter("ab") == Counter("ba") and not (Counter("ab") != Counter("ba")) and Counter("ab") != Counter("abb")
assert Counter("abc").most_common(-1) == [] and Counter("aab").most_common(0) == []
assert Counter("aab").most_common(None) == [("a", 2), ("b", 1)]
assert Counter(None) == Counter()


class Letters:
    def __iter__(self):
        return iter(["x", "y", "x"])


class Countdown:
    def __init__(self):
        self.n = 3

    def __iter__(self):
        return self

    def __next__(self):
        if self.n == 0:
            raise StopIteration
        self.n -= 1
        return self.n


assert Counter(Letters()) == Counter({"x": 2, "y": 1})
assert Counter(Countdown()) == Counter([0, 1, 2])
for f in [lambda: Counter([[1]]), lambda: Counter("ab").most_common(1.5)]:
    try:
        f()
        assert False, "should raise TypeError"
    except TypeError:
        pass
c = Counter()
c["a"] = 1.5
c["b"] += 2
assert c["a"] == 1.5 and c["b"] == 2 and c.total() == 3.5
assert Counter(a=3, b=1) - Counter(a=1, b=2) == Counter(a=2)
assert (Counter(a=1.5) | Counter(a=2)) == Counter(a=2) and (Counter(a=1.5) & Counter(a=2)) == Counter(a=1.5)

# --- format specs ---
for value, spec in [(5, "q"), (5, "s"), (5.5, "d"), ("a", "d"), ("a", "+"), ("a", "=5"), (5, "z"), (5, ".f"), (5, "5dd")]:
    for render in [lambda: format(value, spec), lambda: ("{:" + spec + "}").format(value), lambda: f"{value:{spec}}"]:
        try:
            render()
            assert False, "format(%r, %r) should raise" % (value, spec)
        except ValueError:
            pass
try:
    format(5, "q")
except ValueError as e:
    assert str(e) == "Unknown format code 'q' for object of type 'int'"
for value, spec, want in [(1234567, "_d", "1_234_567"), (255, "#x", "0xff"), (255, "#010x", "0x000000ff"),
                          (-255, "#x", "-0xff"), (255, "#X", "0XFF"), (8, "#o", "0o10"), (5, "#b", "0b101"),
                          (255, "+#x", "+0xff"), (255, ">#10x", "      0xff"), (1234567.891, "_.2f", "1_234_567.89"),
                          (True, "d", "1"), (False, ">5", "    0"), (True, "", "True"), (65, "c", "A"),
                          (97, ">3c", "  a"), (1234567, "n", "1234567"), (2.5, "n", "2.5"), (3.14159, ".2f", "3.14")]:
    assert format(value, spec) == want, (value, spec, format(value, spec))
    assert f"{value:{spec}}" == want

# --- string and bytes literal escapes ---
assert len("\u00e9") == 1 and ord("\u00e9") == 233 and ord("\U0001F600") == 0x1F600
assert "\x41\101\a\b\f\v" == "AA" + chr(7) + chr(8) + chr(12) + chr(11)
assert "\0" == chr(0) and "\08" == chr(0) + "8"
assert """a\tb\x41""" == "a" + chr(9) + "bA"
assert "one\
two" == "onetwo"
assert f"{1}\x41\u00e9" == "1A" + chr(233)
assert b"\xff\x00\101" == bytes([255, 0, 65])
assert b"""x\x41""" == b"xA" and rb"\x41" == b"\\x41" and br"\d" == b"\\d" and Rb"\n" == b"\\n"
assert repr("\u00a0\u2003\x85") == "'\\xa0\\u2003\\x85'" and repr("\u00e9") == "'" + chr(233) + "'"

# --- zip, enumerate, random and itertools argument handling ---
import random

assert list(zip("ab", "xy", strict=True)) == [("a", "x"), ("b", "y")]
for args, message in [(("ab", "x"), "zip() argument 2 is shorter than argument 1"),
                      (("a", "xy"), "zip() argument 2 is longer than argument 1"),
                      (("ab", "xy", "z"), "zip() argument 3 is shorter than arguments 1-2")]:
    try:
        list(zip(*args, strict=True))
        assert False, "strict zip should raise"
    except ValueError as e:
        assert str(e) == message, str(e)
assert list(zip({"a": 1}, "x")) == [("a", "x")] and list(enumerate({"k": 1})) == [(0, "k")]
assert list(zip({"a": 1}.items(), [0])) == [(("a", 1), 0)]

big = random.sample(range(10**12), 4)
assert len(set(big)) == 4 and all(0 <= x < 10**12 for x in big)
assert sorted(random.sample(range(10, 0, -2), 5)) == [2, 4, 6, 8, 10]
assert random.choices([], k=0) == [] and random.choices([5], k=2) == [5, 5]
assert 0 <= random.triangular() <= 1
for f, exc in [(lambda: random.sample([1], 2), ValueError), (lambda: random.choices([]), IndexError),
               (lambda: random.choices([1, 2], weights=[1]), ValueError),
               (lambda: random.choices([1, 2], weights=[0, 0]), ValueError),
               (lambda: itertools.batched([1], 0), ValueError),
               (lambda: itertools.permutations([1, 2], -1), ValueError)]:
    try:
        f()
        assert False, "should raise"
    except exc:
        pass

r = itertools.repeat("x", 2)
assert next(r) == "x" and list(r) == ["x"] and list(itertools.repeat(1, 0)) == []


class BadIter:
    def __iter__(self):
        return 5


try:
    iter(BadIter())
    assert False, "iter() should raise"
except TypeError as e:
    assert str(e) == "iter() returned non-iterator of type 'int'", str(e)

# --- except clauses: aliases, tuple variables and the built-in hierarchy ---
E = KeyError
errs = (KeyError, IndexError)
exc = ValueError


def catches(f, handler_type):
    try:
        f()
    except handler_type:
        return True
    except Exception:
        return False
    return False


assert catches(lambda: {}["x"], E) and not catches(lambda: [][1], E)
assert catches(lambda: [][1], errs) and catches(lambda: random.sample([1], 2), exc)
assert catches(lambda: {}["x"], LookupError) and catches(lambda: [][0], LookupError)
assert catches(lambda: 1 / 0, ArithmeticError) and not catches(lambda: 1 / 0, LookupError)
assert catches(lambda: int("x"), BaseException)
try:
    {}["x"]
except LookupError:
    pass
try:
    raise LookupError("l")
except LookupError as e:
    assert str(e) == "l"
assert isinstance(KeyError("k"), LookupError) and not isinstance(ValueError("v"), LookupError)
assert isinstance(ZeroDivisionError("z"), ArithmeticError) and isinstance(ValueError("v"), BaseException)
