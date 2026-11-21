# A keyword argument that a builtin, type method or library function does not
# take is a TypeError, as in Python; the keywords it does take still work.
import base64
import collections
import copy
import datetime
import functools
import hashlib
import html
import itertools
import json
import math
import random
import re
import statistics
import string
import textwrap
import time
import urllib.parse
import uuid


def rejects(f):
    try:
        f()
    except TypeError:
        return True
    return False


bad = [
    # builtins
    lambda: sorted([3, 1], revrse=True),
    lambda: max(1, 2, kee=len),
    lambda: min([1, 2], defalt=0),
    lambda: enumerate([1], strt=1),
    lambda: len([1], foo=1),
    lambda: sum([1, 2], strt=1),
    lambda: round(1.5, digits=1),
    lambda: int("1", bse=10),
    lambda: zip([1], [2], strickt=True),
    lambda: abs(-1, x=1),
    lambda: list([1], x=1),
    lambda: range(3, step=1),
    lambda: print("x", ennd=""),
    lambda: isinstance(1, int, x=1),
    lambda: repr(1, x=1),
    lambda: pow(2, 3, modd=5),
    lambda: str(1, x=1),
    lambda: ValueError("x", code=1),
    # type methods
    lambda: [3, 1].sort(nope=1),
    lambda: [1].append(2, x=1),
    lambda: "a,b".split(",", maxsplt=1),
    lambda: "abc".upper(x=1),
    lambda: "abc".startswith("a", x=1),
    lambda: "a b".replace(" ", "", cnt=1),
    lambda: "x\ny".splitlines(keepend=True),
    lambda: "a".encode(encodng="utf-8"),
    lambda: {"a": 1}.get("a", dflt=0),
    lambda: {"a": 1}.items(x=1),
    lambda: {1}.add(2, x=1),
    lambda: (1, 2).index(1, x=1),
    lambda: (5).bit_length(x=1),
    lambda: (5).to_bytes(2, byteordr="big"),
    lambda: (1.5).is_integer(x=1),
    lambda: b"ab".decode(encodng="utf-8"),
    lambda: b"a,b".split(b",", maxsplt=1),
    lambda: range(5).index(1, x=1),
    # standard library
    lambda: json.dumps({"a": 1}, indnt=2),
    lambda: json.loads("{}", strct=True),
    lambda: math.sqrt(4, x=1),
    lambda: math.isclose(1.0, 1.0, rel_toll=0.1),
    lambda: re.sub("a", "b", "aa", cnt=1),
    lambda: re.split(",", "a,b", maxsplt=1),
    lambda: re.compile("a", flgs=0),
    lambda: datetime.timedelta(dayz=1),
    lambda: datetime.datetime(2020, 1, 1, hr=1),
    lambda: itertools.zip_longest([1], [2, 3], fill=0),
    lambda: itertools.product([1], rep=2),
    lambda: functools.reduce(lambda a, b: a + b, [1, 2], init=0),
    lambda: collections.deque([1], max_len=2),
    lambda: random.randint(1, 2, x=1),
    lambda: textwrap.wrap("a b c", widht=3),
    lambda: base64.b64encode(b"x", x=1),
    lambda: hashlib.sha256(b"x", x=1),
    lambda: html.escape("<", quot=False),
    lambda: copy.deepcopy([1], mem=None),
    lambda: statistics.mean([1, 2], x=1),
    lambda: urllib.parse.quote("a b", saf="/"),
    lambda: uuid.uuid4(x=1),
    lambda: time.time(x=1),
    lambda: string.Template("$a", x=1),
]
for i, f in enumerate(bad):
    assert rejects(f), "call %d accepted a misspelled keyword" % i
assert len(bad) >= 40

# The same calls with the right keywords.
assert sorted([3, 1, 2], reverse=True) == [3, 2, 1]
assert max(["a", "bbb", "cc"], key=len) == "bbb"
assert min([], default=0) == 0
assert list(enumerate(["a"], start=1)) == [(1, "a")]
assert sum([1, 2], start=10) == 13
assert round(2.567, ndigits=2) == 2.57
assert int("ff", base=16) == 255
assert list(zip([1], [2], strict=True)) == [(1, 2)]
assert pow(2, 10, mod=1000) == 24
assert pow(2, exp=3) == 8
assert str(b"hi", encoding="utf-8") == "hi"
print("", end="")
a = [3, 1, 2]
a.sort(key=lambda x: -x, reverse=False)
assert a == [3, 2, 1]
assert "a,b,c".split(",", maxsplit=1) == ["a", "b,c"]
assert "a,b,c".rsplit(sep=",", maxsplit=1) == ["a,b", "c"]
assert "a b c".replace(" ", "", count=1) == "ab c"
assert "a\tb".expandtabs(tabsize=2) == "a b"
assert "x\ny".splitlines(keepends=True) == ["x\n", "y"]
assert "é".encode(encoding="utf-8") == b"\xc3\xa9"
assert b"hi".decode(encoding="utf-8", errors="strict") == "hi"
assert b"a,b".split(b",", maxsplit=1) == [b"a", b"b"]
assert (1).to_bytes(2, byteorder="little") == b"\x01\x00"
assert (-2).to_bytes(2, "big", signed=True) == b"\xff\xfe"
assert int.from_bytes(b"\xff\xfe", byteorder="big", signed=True) == -2
assert int.from_bytes(b"\x01\x00", "little") == 1
assert json.dumps({"b": 1, "a": 2}, sort_keys=True) == '{"a": 2, "b": 1}'
assert json.dumps([1], indent=2) == "[\n  1\n]"
assert math.isclose(1.0, 1.05, rel_tol=0.1)
assert re.sub("a", "b", "aaa", count=2) == "bba"
assert re.split(",", "a,b,c", maxsplit=1) == ["a", "b,c"]
assert re.match("A", "a", flags=re.IGNORECASE) is not None
assert datetime.timedelta(days=1, hours=1).total_seconds() == 90000
assert datetime.datetime(2020, 1, 2, hour=3).hour == 3
assert list(itertools.zip_longest([1], [2, 3], fillvalue=0)) == [(1, 2), (0, 3)]
assert list(itertools.product([1, 2], repeat=2)) == [(1, 1), (1, 2), (2, 1), (2, 2)]
assert functools.reduce(lambda x, y: x + y, [1, 2], 10) == 13
assert list(collections.deque([1, 2, 3], maxlen=2)) == [2, 3]
assert textwrap.wrap("aaa bbb", width=3) == ["aaa", "bbb"]
assert html.escape("<'", quote=False) == "&lt;'"
assert urllib.parse.quote("a/b c", safe="") == "a%2Fb%20c"
assert len(hashlib.sha256(b"x").hexdigest()) == 64
random.seed(1)
assert 1 <= random.randint(a=1, b=3) <= 3

# Callables that take arbitrary keywords still take them.
assert dict(a=1, b=2) == {"a": 1, "b": 2}
d = {}
d.update(x=1)
assert d == {"x": 1}
assert "{who} {n}".format(who="a", n=1) == "a 1"
assert functools.partial(lambda x, y=0: x + y, y=2)(1) == 3
assert collections.Counter(a=2)["a"] == 2
c = collections.Counter()
c.update(b=3)
assert c["b"] == 3
assert collections.OrderedDict(z=1)["z"] == 1
assert collections.defaultdict(int, q=4)["q"] == 4
assert string.Template("$who").substitute(who="x") == "x"
assert string.Template("$who $n").safe_substitute(who="y") == "y $n"
P = collections.namedtuple("P", ["x", "y"])
assert P(x=1, y=2)._replace(y=3) == P(1, 3)


def user(**kw):
    return sorted(kw)


assert user(anything=1, goes=2) == ["anything", "goes"]
assert sorted(["b", "A"], key=str.lower) == ["A", "b"]

print("OK")
