# Library calls raise the exception types Python raises, so scripts can catch
# them with `except ValueError:` / `except TypeError:` as they would in
# Python, and the valid neighbours of each bad call still work (so a library
# that rejected too much would fail here too). Runs unchanged under python3.
import base64, collections, copy, datetime, difflib, functools, hashlib, hmac
import io, itertools, json, math, random, re, secrets, shlex, statistics
import textwrap, time, uuid
import urllib.parse

cases = 0


def raises(exc, fn, label):
    global cases
    cases += 1
    try:
        fn()
    except exc:
        return
    except Exception as e:
        assert False, label + ": raised " + type(e).__name__ + " not " + str(exc)
    assert False, label + ": did not raise " + str(exc)


def equals(expected, fn, label):
    global cases
    cases += 1
    got = fn()
    assert got == expected, label + ": got " + repr(got) + ", want " + repr(expected)


D = datetime.datetime
DT = datetime.date
TD = datetime.timedelta

# ---- datetime: fields out of range are a ValueError, not a rollover
raises(ValueError, lambda: D(2020, 2, 30), "datetime(2020, 2, 30)")
raises(ValueError, lambda: D(2021, 2, 29), "datetime(2021, 2, 29)")
equals("2020-02-29 00:00:00", lambda: str(D(2020, 2, 29)), "leap day")
raises(ValueError, lambda: DT(2020, 13, 1), "date(2020, 13, 1)")
raises(ValueError, lambda: DT(2020, 0, 1), "date(2020, 0, 1)")
raises(ValueError, lambda: DT(2020, 1, 0), "date(2020, 1, 0)")
raises(ValueError, lambda: DT(0, 1, 1), "date(0, 1, 1)")
raises(ValueError, lambda: DT(10000, 1, 1), "date(10000, 1, 1)")
equals("2020-12-31", lambda: str(DT(2020, 12, 31)), "date(2020, 12, 31)")
equals("0001-01-01", lambda: str(DT(1, 1, 1)), "date(1, 1, 1)")
equals("9999-12-31 23:59:59", lambda: str(D(9999, 12, 31, 23, 59, 59)), "datetime max")
raises(ValueError, lambda: D(2020, 1, 1, 24), "hour 24")
raises(ValueError, lambda: D(2020, 1, 1, -1), "hour -1")
raises(ValueError, lambda: D(2020, 1, 1, 0, 60), "minute 60")
raises(ValueError, lambda: D(2020, 1, 1, 0, 0, 60), "second 60")
raises(ValueError, lambda: D(2020, 1, 1, 0, 0, 0, 1000000), "microsecond 1000000")
equals(999999, lambda: D(2020, 1, 1, 23, 59, 59, 999999).microsecond, "microsecond max")
raises(TypeError, lambda: D(2020, 1, 2, 3, hour=4), "hour given twice")
raises(TypeError, lambda: D(2020, 1, 1, month=2), "month given twice")
raises(TypeError, lambda: D(2020, 1), "datetime missing day")
raises(TypeError, lambda: DT(2020, 1, 1, 1), "date too many arguments")
raises(TypeError, lambda: D("2020", 1, 1), "datetime str year")
raises(TypeError, lambda: D(2020, 1, 1, foo=1), "datetime unknown keyword")
equals("2020-01-02 00:00:00", lambda: str(D(2020, 1, day=2)), "datetime day keyword")
equals("2020-01-02 00:00:00", lambda: str(D(year=2020, month=1, day=2)), "datetime all keywords")
equals("2020-01-05", lambda: str(DT(2020, 1, day=5)), "date day keyword")

# ---- replace: positional fields, validation, duplicates
equals("2021-01-02 00:00:00", lambda: str(D(2020, 1, 2).replace(2021)), "dt.replace(2021)")
equals("2021-05-02 00:00:00", lambda: str(D(2020, 1, 2).replace(2021, 5)), "dt.replace(2021, 5)")
equals("2021-05-02", lambda: str(DT(2020, 1, 2).replace(2021, 5)), "d.replace(2021, 5)")
equals("2020-01-02 05:00:00", lambda: str(D(2020, 1, 2).replace(hour=5)), "dt.replace(hour=5)")
raises(ValueError, lambda: D(2020, 1, 2).replace(month=2, day=31), "replace to 31 February")
raises(ValueError, lambda: D(2020, 1, 2).replace(hour=25), "replace hour 25")
raises(ValueError, lambda: DT(2020, 1, 2).replace(month=13), "date replace month 13")
raises(TypeError, lambda: D(2020, 1, 2).replace(2021, year=2), "replace year twice")
raises(TypeError, lambda: D(2020, 1, 2).replace(foo=5), "replace unknown keyword")

# ---- parsing dates
raises(ValueError, lambda: D.strptime("x", "%Y"), "strptime bad text")
raises(ValueError, lambda: D.strptime("2020-13-01", "%Y-%m-%d"), "strptime month 13")
equals(2020, lambda: D.strptime("2020", "%Y").year, "strptime year")
raises(ValueError, lambda: D.fromisoformat("x"), "datetime.fromisoformat bad")
raises(ValueError, lambda: D.fromisoformat("2020-02-30"), "datetime.fromisoformat 30 February")
raises(ValueError, lambda: DT.fromisoformat("x"), "date.fromisoformat bad")
equals("2020-01-02", lambda: str(DT.fromisoformat("2020-01-02")), "date.fromisoformat")
raises(ValueError, lambda: time.strptime("x", "%Y"), "time.strptime bad")
equals(2020, lambda: time.strptime("2020", "%Y")[0], "time.strptime year")

# ---- timedelta and datetime arithmetic
equals("1 day, 0:00:00", lambda: str(abs(TD(-1))), "abs(timedelta(-1))")
equals("3:00:00", lambda: str(abs(TD(hours=3))), "abs(timedelta(hours=3))")
raises(TypeError, lambda: TD(foo=1), "timedelta unknown keyword")
equals("3652058 days, 0:00:00", lambda: str(D(9999, 12, 31) - D(1, 1, 1)), "datetime full-range difference")
equals("2020-03-08 12:00:00", lambda: str(D(2020, 3, 7, 12) + TD(days=1)), "datetime + timedelta")
raises(OverflowError, lambda: D(9999, 12, 31) + TD(days=1), "datetime past year 9999")

# ---- math: domain errors and overflow
raises(ValueError, lambda: math.sqrt(-1), "math.sqrt(-1)")
equals(2.0, lambda: math.sqrt(4), "math.sqrt(4)")
raises(ValueError, lambda: math.acos(2), "math.acos(2)")
equals(0.0, lambda: math.acos(1), "math.acos(1)")
raises(ValueError, lambda: math.asin(-2), "math.asin(-2)")
raises(ValueError, lambda: math.log(0), "math.log(0)")
raises(ValueError, lambda: math.log(-1), "math.log(-1)")
equals(3.0, lambda: math.log(8, 2), "math.log(8, 2)")
raises(ValueError, lambda: math.log10(0), "math.log10(0)")
raises(ValueError, lambda: math.log2(-1), "math.log2(-1)")
raises(ValueError, lambda: math.log1p(-1), "math.log1p(-1)")
raises(ValueError, lambda: math.pow(0, -1), "math.pow(0, -1)")
raises(ValueError, lambda: math.pow(-8, 0.5), "math.pow(-8, 0.5)")
equals(8.0, lambda: math.pow(2, 3), "math.pow(2, 3)")
raises(OverflowError, lambda: math.pow(10, 400), "math.pow(10, 400)")
raises(ValueError, lambda: math.fmod(1, 0), "math.fmod(1, 0)")
equals(1.0, lambda: math.fmod(7, 3), "math.fmod(7, 3)")
raises(ValueError, lambda: math.factorial(-1), "math.factorial(-1)")
raises(TypeError, lambda: math.factorial(1.5), "math.factorial(1.5)")
equals(120, lambda: math.factorial(5), "math.factorial(5)")
raises(ValueError, lambda: math.comb(2, -1), "math.comb(2, -1)")
raises(TypeError, lambda: math.comb(5.0, 2), "math.comb(5.0, 2)")
equals(10, lambda: math.comb(5, 2), "math.comb(5, 2)")
equals(0, lambda: math.comb(2, 5), "math.comb(2, 5)")
raises(ValueError, lambda: math.perm(-1), "math.perm(-1)")
equals(20, lambda: math.perm(5, 2), "math.perm(5, 2)")
raises(ValueError, lambda: math.isqrt(-1), "math.isqrt(-1)")
raises(TypeError, lambda: math.isqrt(1.5), "math.isqrt(1.5)")
equals(4, lambda: math.isqrt(17), "math.isqrt(17)")
equals(3037000499, lambda: math.isqrt(9223372036854775807), "math.isqrt(max int)")
raises(OverflowError, lambda: math.exp(100000), "math.exp(100000)")
equals(1.0, lambda: math.exp(0), "math.exp(0)")
raises(TypeError, lambda: math.gcd(1.5, 2), "math.gcd(1.5, 2)")
equals(6, lambda: math.gcd(12, 18), "math.gcd(12, 18)")
equals(12, lambda: math.lcm(4, 6), "math.lcm(4, 6)")
raises(ValueError, lambda: math.cos(float("inf")), "math.cos(inf)")
raises(ValueError, lambda: math.remainder(1, 0), "math.remainder(1, 0)")
raises(OverflowError, lambda: math.floor(float("inf")), "math.floor(inf)")
raises(ValueError, lambda: math.floor(float("nan")), "math.floor(nan)")
equals(3, lambda: math.ceil(2.1), "math.ceil(2.1)")
raises(ValueError, lambda: math.isclose(1, 1, rel_tol=-1), "math.isclose negative tolerance")
raises(ValueError, lambda: math.dist([1], [1, 2]), "math.dist dimension mismatch")
raises(ValueError, lambda: math.atanh(1), "math.atanh(1)")
raises(ValueError, lambda: math.acosh(0), "math.acosh(0)")
equals((0.5, 2.0), lambda: math.modf(2.5), "math.modf(2.5)")
equals((0.5, 4), lambda: math.frexp(8), "math.frexp(8)")
raises(ValueError, lambda: math.gamma(0), "math.gamma(0)")

# ---- json
raises(TypeError, lambda: json.dumps({1, 2}), "json.dumps(set)")
raises(TypeError, lambda: json.dumps(len), "json.dumps(builtin)")
raises(TypeError, lambda: json.dumps(b"x"), "json.dumps(bytes)")
raises(TypeError, lambda: json.dumps(D(2020, 1, 2)), "json.dumps(datetime)")
raises(TypeError, lambda: json.dumps({(1, 2): 3}), "json.dumps tuple key")
equals('[1, 2]', lambda: json.dumps((1, 2)), "json.dumps(tuple)")
equals('{"1": 2}', lambda: json.dumps({1: 2}), "json.dumps int key")
equals('{"true": 1, "null": 2}', lambda: json.dumps({True: 1, None: 2}), "json.dumps bool/None keys")
equals('{"a": 2, "b": 1}', lambda: json.dumps(collections.Counter({"a": 2, "b": 1})), "json.dumps(Counter)")
equals('[1, 2]', lambda: json.dumps(collections.namedtuple("P", "x y")(1, 2)), "json.dumps(namedtuple)")
equals('[3]', lambda: json.dumps({3}, default=list), "json.dumps default=list")
equals('["2020-01-02 00:00:00"]', lambda: json.dumps([D(2020, 1, 2)], default=str), "json.dumps default=str")
raises(ZeroDivisionError, lambda: json.dumps({1}, default=lambda o: 1 / 0), "json.dumps default raises")
equals('{\n  "b":1,\n  "a":[\n    1,\n    2\n  ]\n}', lambda: json.dumps({"b": 1, "a": [1, 2]}, indent=2, separators=(",", ":")), "json.dumps indent + separators")
equals('{"b":1,"a":[1,2]}', lambda: json.dumps({"b": 1, "a": [1, 2]}, separators=(",", ":")), "json.dumps compact")
equals('[\n1\n]', lambda: json.dumps([1], indent=-1), "json.dumps negative indent")
raises(TypeError, lambda: json.loads(5), "json.loads(5)")
raises(TypeError, lambda: json.loads(None), "json.loads(None)")
raises(ValueError, lambda: json.loads("{"), "json.loads('{')")
raises(ValueError, lambda: json.loads(""), "json.loads('')")
equals([1], lambda: json.loads(b"[1]"), "json.loads(bytes)")
equals([1, 2.5, None], lambda: json.loads("[1, 2.5, null]"), "json.loads list")

# ---- random
raises(ValueError, lambda: random.randint(5, 1), "random.randint(5, 1)")
equals(1, lambda: random.randint(1, 1), "random.randint(1, 1)")
raises(ValueError, lambda: random.sample([1, 2], 5), "random.sample too large")
equals(2, lambda: len(random.sample([1, 2], 2)), "random.sample full")
raises(IndexError, lambda: random.choice([]), "random.choice([])")
raises(IndexError, lambda: random.choice(()), "random.choice(())")
equals(7, lambda: random.choice([7]), "random.choice([7])")
equals(1, lambda: random.choice((1,)), "random.choice((1,))")
equals("é", lambda: random.choice("é"), "random.choice non-ASCII")
raises(ValueError, lambda: random.randrange(0), "random.randrange(0)")
raises(ValueError, lambda: random.randrange(1, 5, 0), "random.randrange step 0")
raises(TypeError, lambda: random.randrange(), "random.randrange()")
equals([], lambda: random.choices([1], k=-1), "random.choices negative k")
raises(ZeroDivisionError, lambda: random.expovariate(0), "random.expovariate(0)")
raises(ValueError, lambda: random.betavariate(0, 1), "random.betavariate(0, 1)")

# ---- statistics (StatisticsError is a ValueError)
raises(ValueError, lambda: statistics.mean([]), "statistics.mean([])")
equals(2, lambda: statistics.mean([1, 2, 3]), "statistics.mean int")
equals("int", lambda: type(statistics.mean([1, 2, 3])).__name__, "statistics.mean int type")
equals(1.5, lambda: statistics.mean([1, 2]), "statistics.mean fractional")
equals(2.0, lambda: statistics.mean([1.0, 3.0]), "statistics.mean float")
raises(ValueError, lambda: statistics.median([]), "statistics.median([])")
equals(2.0, lambda: statistics.median([1, 3]), "statistics.median even")
equals(2, lambda: statistics.median([1, 2, 3]), "statistics.median odd")
equals(2, lambda: statistics.median((3, 1, 2)), "statistics.median tuple")
raises(ValueError, lambda: statistics.mode([]), "statistics.mode([])")
equals(2, lambda: statistics.mode([2, 1, 1, 2]), "statistics.mode first of ties")
raises(ValueError, lambda: statistics.stdev([1]), "statistics.stdev([1])")
equals(1.0, lambda: statistics.stdev([1, 2, 3]), "statistics.stdev")
raises(ValueError, lambda: statistics.variance([1]), "statistics.variance([1])")
equals(1, lambda: statistics.variance([1, 2, 3]), "statistics.variance int")
equals(1.25, lambda: statistics.pvariance([1, 2, 3, 4]), "statistics.pvariance")
raises(ValueError, lambda: statistics.pstdev([]), "statistics.pstdev([])")
raises(ValueError, lambda: statistics.fmean([]), "statistics.fmean([])")
raises(ValueError, lambda: statistics.geometric_mean([-1, 2]), "statistics.geometric_mean negative")
raises(ValueError, lambda: statistics.harmonic_mean([-1]), "statistics.harmonic_mean negative")
equals(0, lambda: statistics.harmonic_mean([1, 0]), "statistics.harmonic_mean zero")
equals(1, lambda: statistics.median_low([1, 3]), "statistics.median_low")
equals(3, lambda: statistics.median_high([1, 3]), "statistics.median_high")

# ---- collections
raises(TypeError, lambda: collections.Counter("aab").most_common(1, n=1), "most_common n twice")
equals([("a", 2)], lambda: collections.Counter("aab").most_common(1), "most_common(1)")
raises(TypeError, lambda: collections.deque([1], iterable=[2]), "deque iterable twice")
equals([1, 2], lambda: list(collections.deque(iterable=[1, 2])), "deque(iterable=)")
raises(ValueError, lambda: collections.namedtuple("P", [1]), "namedtuple int field")
raises(ValueError, lambda: collections.namedtuple("P", "x 1y"), "namedtuple bad identifier")
raises(ValueError, lambda: collections.namedtuple("P", "x x"), "namedtuple duplicate field")
equals(1, lambda: collections.namedtuple("P", "x y")(1, 2).x, "namedtuple field")
raises(IndexError, lambda: collections.deque().pop(), "deque().pop()")

# ---- encodings, hashes, ids
raises(ValueError, lambda: uuid.UUID("nope"), "uuid.UUID('nope')")
equals("12345678-1234-5678-1234-567812345678", lambda: str(uuid.UUID("12345678123456781234567812345678")), "uuid.UUID hex")
raises(ValueError, lambda: hashlib.new("nope"), "hashlib.new('nope')")
equals("ca978112", lambda: hashlib.new("sha256", b"a").hexdigest()[:8], "hashlib.new sha256")
equals("d41d8cd9", lambda: hashlib.new("md5").hexdigest()[:8], "hashlib.new md5")
raises(ValueError, lambda: hmac.new(b"k", b"m", "nope"), "hmac unknown digest")
equals(True, lambda: hmac.compare_digest(b"a", b"a"), "hmac.compare_digest bytes")
raises(TypeError, lambda: hmac.compare_digest("a", 5), "hmac.compare_digest mixed")
equals(b"", lambda: base64.b64decode("!!!"), "b64decode discards non-alphabet")
equals(b"a", lambda: base64.b64decode("Y Q = ="), "b64decode ignores spaces")
equals(b"a", lambda: base64.b64decode("YQ=="), "b64decode")
raises(ValueError, lambda: base64.b64decode("!!!", validate=True), "b64decode validate")
raises(ValueError, lambda: base64.b64decode("YQ"), "b64decode missing padding")
equals(b"\xfb\xff", lambda: base64.urlsafe_b64decode("-_8="), "urlsafe_b64decode")
raises(ValueError, lambda: secrets.token_hex(-1), "secrets.token_hex(-1)")
equals(0, lambda: len(secrets.token_hex(0)), "secrets.token_hex(0)")
raises(TypeError, lambda: secrets.token_hex("x"), "secrets.token_hex('x')")
raises(ValueError, lambda: secrets.randbelow(0), "secrets.randbelow(0)")
equals(0, lambda: secrets.randbelow(1), "secrets.randbelow(1)")
equals(0, lambda: secrets.randbits(0), "secrets.randbits(0)")
raises(IndexError, lambda: secrets.choice([]), "secrets.choice([])")
raises(TypeError, lambda: secrets.compare_digest("a", 5), "secrets.compare_digest mixed")

# ---- re
try:
    re.compile("(")
    assert False, "re.compile('(') did not raise"
except re.error:
    cases += 1
raises(Exception, lambda: re.search("[", "x"), "re.search('[')")
raises(IndexError, lambda: re.match("a", "a").group(5), "group out of range")
raises(IndexError, lambda: re.match("a", "a").start(3), "start out of range")
equals(None, lambda: re.match("(a)(b)?", "a").group(2), "unmatched group is None")
equals(("a", None), lambda: re.match("(a)(b)?", "a").groups(), "groups with unmatched")
equals((-1, -1), lambda: re.match("(a)(b)?", "a").span(2), "span of unmatched group")
equals(2, lambda: re.match("x(a)", "xa").end(1), "end(1)")
equals((1, 2), lambda: re.search("b", "éb").span(), "span in characters")
raises(TypeError, lambda: re.sub("a", "b", "aaa", count="x"), "re.sub count type")
raises(TypeError, lambda: re.match(), "re.match()")
try:
    re.sub("(a)", "\\9", "a")
    assert False, "re.sub bad group reference did not raise"
except re.error:
    cases += 1
equals("xab", lambda: re.sub("(a)", r"\1b", "xa"), "re.sub \\1 then text")
equals("cost $x", lambda: re.sub("a", "cost $x", "a"), "re.sub literal $")
equals("x\ny", lambda: re.sub("a", r"x\ny", "a"), "re.sub \\n escape")
equals("bba", lambda: re.sub("a", "b", "aaa", count=2), "re.sub count")

# ---- itertools / functools / textwrap / misc
raises(ValueError, lambda: list(itertools.combinations([1, 2], -1)), "combinations negative r")
raises(ValueError, lambda: list(itertools.product([1], repeat=-1)), "product negative repeat")
equals([(1, 2), (1, 3), (2, 3)], lambda: list(itertools.combinations([1, 2, 3], 2)), "combinations")
raises(TypeError, lambda: list(itertools.accumulate([1, "a"])), "accumulate int + str")
raises(TypeError, lambda: list(itertools.starmap(pow, [1])), "starmap non-sequence")
raises(TypeError, lambda: functools.reduce(pow), "reduce missing iterable")
raises(TypeError, lambda: functools.partial(), "functools.partial()")
raises(TypeError, lambda: functools.lru_cache("x"), "lru_cache('x')")
equals(8, lambda: functools.partial(pow, 2)(3), "functools.partial")
raises(TypeError, lambda: textwrap.dedent(), "textwrap.dedent()")
raises(TypeError, lambda: textwrap.indent("a"), "textwrap.indent missing prefix")
equals(["abc", "def"], lambda: textwrap.wrap("abc def", 3), "textwrap.wrap")
raises(TypeError, lambda: copy.deepcopy(), "copy.deepcopy()")
equals([1, [2]], lambda: copy.deepcopy([1, [2]]), "copy.deepcopy")
raises(ValueError, lambda: shlex.split('"a'), "shlex.split unclosed quote")
equals(["a", "b c"], lambda: shlex.split("a 'b c'"), "shlex.split")
raises(ValueError, lambda: io.StringIO().seek(-1), "StringIO.seek(-1)")
raises(TypeError, lambda: io.StringIO().seek(), "StringIO.seek()")
equals("ab", lambda: io.StringIO("ab").read(), "StringIO.read")
raises(ValueError, lambda: difflib.get_close_matches("a", ["a"], n=0), "get_close_matches n=0")
raises(ValueError, lambda: difflib.get_close_matches("a", ["a"], cutoff=2), "get_close_matches cutoff 2")
equals(["ab"], lambda: difflib.get_close_matches("ab", ["ab", "xy"]), "get_close_matches")
raises(ValueError, lambda: time.sleep(-1), "time.sleep(-1)")
raises(IndexError, lambda: time.gmtime(0)[9], "struct_time index 9")
equals(1970, lambda: time.gmtime(0)[0], "struct_time index 0")

# ---- urllib.parse is lenient where Python is
equals("%zz", lambda: urllib.parse.unquote("%zz"), "unquote bad escape")
equals("a %zz", lambda: urllib.parse.unquote_plus("a+%zz"), "unquote_plus bad escape")
equals("A", lambda: urllib.parse.unquote("%41"), "unquote")
equals({"b": ["1", "3"], "a": ["2"]}, lambda: urllib.parse.parse_qs("b=1&a=2&b=3"), "parse_qs")
equals([("b", "1"), ("a", "2"), ("b", "3")], lambda: urllib.parse.parse_qsl("b=1&a=2&b=3"), "parse_qsl order")
equals({"a": ["1"]}, lambda: urllib.parse.parse_qs("a=1&b=&c"), "parse_qs drops blanks")
equals([("a", "%zz")], lambda: urllib.parse.parse_qsl("a=%zz"), "parse_qsl bad escape")
raises(ValueError, lambda: urllib.parse.urlparse("http://[::1"), "urlparse bad IPv6")

assert cases >= 80, cases
# Library exception classes are catchable by their own name and as ValueError.
import statistics as _st
for _exc in (_st.StatisticsError, ValueError):
    try:
        _st.mean([])
        assert False
    except _exc:
        pass
assert issubclass(_st.StatisticsError, ValueError)

print("OK")
