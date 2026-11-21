# int()/float() parsing, bytes gaps, json sort_keys with mixed keys,
# isocalendar() results and uuid.UUID construction, as in Python.
import datetime
import json
import uuid

# int() and float() accept Unicode decimal digits, underscores and whitespace.
assert int("١٢٣") == 123
assert int("１２３") == 123
assert int("߁") == 1
assert float("٣.٥") == 3.5
assert int("1_000") == 1000
assert int(" 12\n") == 12
assert int("  -7  ") == -7
assert int("0x1f", 16) == 31
assert int("0b101", 0) == 5
assert int("0o17", 0) == 15
assert int("1_0", 0) == 10
assert int("zz", 36) == 1295
assert float("1_0.5") == 10.5
assert float("infinity") == float("inf")
assert float("-Infinity") == float("-inf")
assert str(float("-nan")) == "nan"
assert float(" 1.5 ") == 1.5
assert float(".5") == 0.5
for bad in ["1__0", "_1", "1_", "١٫٥"]:
    try:
        int(bad)
        assert False, bad
    except ValueError:
        pass
try:
    int("010", 0)
    assert False
except ValueError:
    pass

# bytes
assert b"hello".index(b"l") == 2
assert b"hello".index(108) == 2
assert b"hello".count(b"l") == 2
assert b"hello".count(108) == 2
assert b"hello".rfind(b"l") == 3
assert b"hello".rindex(b"l") == 3
assert b",".join([b"a", b"b"]) == b"a,b"
assert b"ab1".isalnum() and b"ab".isalpha() and b"12".isdigit() and b" ".isspace()
assert b"AB".isupper() and b"ab".islower() and b"Ab".istitle()
assert b"hello"[1:3] == b"el" and b"hello"[1] == 101 and b"hello"[-1] == 111 and b"hello"[::2] == b"hlo"
assert b"a" < b"b" and b"a" <= b"a" and b"b" > b"abc"
assert b"a" + b"b" == b"ab" and b"ab" * 3 == b"ababab" and 2 * b"ab" == b"abab"
assert 97 in b"abc" and b"bc" in b"abc" and 120 not in b"abc"
assert (b"a" == "a") is False
assert sorted([b"b", b"a", b"ab"]) == [b"a", b"ab", b"b"]
assert max([b"b", b"a"]) == b"b"
assert list(b"ab") == [97, 98]

# json.dumps sort_keys sorts the original keys.
assert json.dumps({1: "a", "b": 2}) == '{"1": "a", "b": 2}'
try:
    json.dumps({1: "a", "b": 2}, sort_keys=True)
    assert False
except TypeError:
    pass
assert json.dumps({2: "a", 1: 2}, sort_keys=True) == '{"1": 2, "2": "a"}'
assert json.dumps({10: 1, 9: 2}, sort_keys=True) == '{"9": 2, "10": 1}'
assert json.dumps({True: 1, 2: 3}, sort_keys=True) == '{"true": 1, "2": 3}'
assert json.dumps({1.5: 1, 2: 3}, sort_keys=True) == '{"1.5": 1, "2": 3}'
assert json.dumps({"b": 1, "a": 2}, sort_keys=True) == '{"a": 2, "b": 1}'

# isocalendar() is a named tuple of year, week and weekday.
ic = datetime.date(2024, 5, 6).isocalendar()
assert ic == (2024, 19, 1) and (2024, 19, 1) == ic
assert (ic.year, ic.week, ic.weekday) == (2024, 19, 1)
assert ic[0] == 2024 and ic[1:] == (19, 1) and len(ic) == 3
y, w, d = ic
assert (y, w, d) == (2024, 19, 1)
assert isinstance(ic, tuple)
assert ic < (2025, 1, 1)
assert sorted([ic, (2024, 1, 1)])[0] == (2024, 1, 1)
assert datetime.datetime(2021, 1, 3).isocalendar().week == 53
assert datetime.datetime(2021, 1, 3).isocalendar().year == 2020

# uuid.UUID
u = uuid.UUID("12345678-1234-5678-1234-567812345678")
assert str(u) == "12345678-1234-5678-1234-567812345678"
assert repr(u) == "UUID('12345678-1234-5678-1234-567812345678')"
assert u.hex == "12345678123456781234567812345678"
assert u.bytes == b"\x124Vx\x124Vx\x124Vx\x124Vx"
assert uuid.UUID(bytes=u.bytes) == u
assert uuid.UUID(hex="12345678123456781234567812345678") == u
assert uuid.UUID("{12345678-1234-5678-1234-567812345678}") == u
assert uuid.UUID("urn:uuid:12345678-1234-5678-1234-567812345678") == u
assert uuid.UUID(bytes_le=u.bytes) == uuid.UUID("78563412-3412-7856-1234-567812345678")
assert uuid.UUID(bytes=b"\x00" * 16) == uuid.UUID("00000000-0000-0000-0000-000000000000")
assert u.version is None
assert u.variant == uuid.RESERVED_NCS == "reserved for NCS compatibility"
v = uuid.UUID("12345678-1234-5678-9234-567812345678")
assert v.version == 5 and v.variant == uuid.RFC_4122
assert uuid.UUID("12345678123456781234567812345678", version=4) == uuid.UUID("12345678-1234-4678-9234-567812345678")
assert v.fields == (305419896, 4660, 22136, 146, 52, 95073701484152)
assert (v.time_low, v.time_mid, v.time_hi_version) == (305419896, 4660, 22136)
assert (v.clock_seq_hi_variant, v.clock_seq_low, v.node, v.clock_seq) == (146, 52, 95073701484152, 4660)
assert v.time == 466142576285865592
assert u.urn == "urn:uuid:12345678-1234-5678-1234-567812345678"
assert hash(u) == hash(uuid.UUID(str(u)))
assert {u: 1}[uuid.UUID(str(u))] == 1
assert u < uuid.UUID("22345678-1234-5678-1234-567812345678")
assert sorted([v, u]) == [u, v]
assert (u == "12345678-1234-5678-1234-567812345678") is False
assert str(uuid.uuid5(uuid.NAMESPACE_DNS, "python.org")) == "886313e1-3b8a-5372-9b90-0c9aee199e5d"
assert str(uuid.uuid3(uuid.NAMESPACE_URL, "http://x")) == str(uuid.uuid3(uuid.UUID("6ba7b811-9dad-11d1-80b4-00c04fd430c8"), "http://x"))
assert uuid.NAMESPACE_DNS == uuid.UUID("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
assert uuid.uuid4().version == 4 and len(uuid.uuid4().bytes) == 16
for kw in [{}, {"bytes": b"x"}]:
    try:
        uuid.UUID(**kw)
        assert False
    except (TypeError, ValueError):
        pass

print("OK")
