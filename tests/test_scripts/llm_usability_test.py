# Round-20 LLM-usability battery: patterns Python-trained models emit
# constantly. Every behavior was diffed against CPython 3.14.

# --- Unhashable keys are TypeErrors, not silent dict corruption --------------
try:
    hash([1])
    assert False, "expected TypeError"
except TypeError as e:
    assert "unhashable type" in str(e)

try:
    d = {[1, 2]: "v"}
    assert False, "expected TypeError"
except TypeError:
    pass

try:
    s = {[3]}
    assert False, "expected TypeError"
except TypeError:
    pass

try:
    d = {([1],): "x"}  # tuples of unhashables are unhashable too
    assert False, "expected TypeError"
except TypeError:
    pass

class Plain:
    pass

p = Plain()
assert len({p, p}) == 1  # plain instances identity-hash, as in Python
assert {p: "v"}[p] == "v"

class EqNoHash:
    def __eq__(self, other):
        return True

try:
    {EqNoHash()}
    assert False, "expected TypeError"
except TypeError:
    pass

# --- File errors are the OSError family ----------------------------------------
import os

try:
    os.read_file("/nonexistent-scriptling-probe")
    assert False, "expected FileNotFoundError"
except FileNotFoundError:
    pass

try:
    os.listdir("/nonexistent-scriptling-probe")
except OSError:
    pass

try:
    os.read_file("/nonexistent-scriptling-probe")
except Exception:
    pass  # base Exception catches the family

# --- Custom exception classes ---------------------------------------------------
class MyError(Exception):
    pass

try:
    raise MyError("custom boom")
except MyError as e:
    assert str(e) == "custom boom"

class MyValueError(ValueError):
    pass

try:
    raise MyValueError("v")
except ValueError as e:  # base class catches the subclass
    assert str(e) == "v"

class GrandErr(Exception):
    pass

class MidErr(GrandErr):
    pass

try:
    raise MidErr("deep")
except GrandErr:
    pass

try:
    raise MyError("x")
except Exception as e:
    assert isinstance(e, MyError)

raise_result = None
try:
    raise MyError("multi", 42)
except MyError as e:
    assert e.args == ("multi", 42)
    assert str(e) == "('multi', 42)"

class CodedError(Exception):
    def __init__(self, code):
        self.code = code

try:
    raise CodedError(7)
except CodedError as e:
    assert e.code == 7

# raise with the class, no call
try:
    raise MyError
except MyError:
    pass
try:
    raise ValueError
except ValueError:
    pass

# --- Exception introspection ------------------------------------------------------
try:
    raise ValueError("v", 42)
except ValueError as e:
    assert e.args == ("v", 42)
    assert len(e.args) == 2

try:
    raise ValueError("v")
except ValueError as e:
    assert type(e).__name__ == "ValueError"
    assert e.args == ("v",)

try:
    x = 1 / 0
except ZeroDivisionError as e:
    assert type(e).__name__ == "ZeroDivisionError"

# --- str.split(None, maxsplit) -----------------------------------------------------
s = "a b  c   d"
assert s.split(None, 1) == ["a", "b  c   d"]
assert s.split(None) == ["a", "b", "c", "d"]
assert s.split(None, 0) == ["a b  c   d"]
assert s.rsplit(None, 1) == ["a b  c", "d"]
assert "  x  ".split() == ["x"]

# --- bytes.join ---------------------------------------------------------------------
assert b"-".join([b"a", b"b", b"c"]) == b"a-b-c"
assert b"".join((b"x", b"y")) == b"xy"
try:
    "-".join(["a"])  # str join stays str; bytes join wants bytes
except TypeError:
    pass
assert "-".join(["a", "b"]) == "a-b"

# --- json.JSONDecodeError -----------------------------------------------------------
import json

try:
    json.loads("{bad")
except json.JSONDecodeError:
    pass
try:
    json.loads("{bad")
except ValueError:  # hierarchy: JSONDecodeError is a ValueError
    pass

# --- f-string nested spec fields -----------------------------------------------------
assert f"{5:>{2}}" == " 5"
assert f"{5:>{1+2}}" == "  5"
width = 6
assert f"{7:>{width}}" == "     7"
width, prec = 10, 2
assert f"{3.14159:{width}.{prec}f}" == "      3.14"

# --- Exception constructor coverage ---------------------------------------------------
for exc_cls, name in (
    (NotImplementedError, "NotImplementedError"),
    (FileNotFoundError, "FileNotFoundError"),
    (TimeoutError, "TimeoutError"),
    (ModuleNotFoundError, "ModuleNotFoundError"),
    (UnicodeDecodeError, "UnicodeDecodeError"),
    (AssertionError, "AssertionError"),
):
    try:
        raise exc_cls("x")
    except exc_cls as e:
        assert type(e).__name__ == name

try:
    raise NotImplementedError("todo")
except NotImplementedError as e:
    assert str(e) == "todo"

# --- Review fixes: UUID hashing, csv coercion + Dict classes, int bytes ------
import csv
import io
import uuid

u = uuid.uuid4()
d = {u: 1}
assert d[u] == 1
assert len({u, u}) == 1
u2 = uuid.UUID(str(u))
assert u == u2  # value equality
assert hash(u) == hash(u2)  # and value hashing

buf = io.StringIO()
w = csv.writer(buf)
w.writerow([1, "a b", 2.5, True, None])
assert buf.getvalue() == "1,a b,2.5,True,\r\n"

buf2 = io.StringIO()
dw = csv.DictWriter(buf2, ["name", "age"])
dw.writeheader()
dw.writerow({"name": "x", "age": 7})
dw.writerow({"name": "y"})
assert buf2.getvalue() == "name,age\r\nx,7\r\ny,\r\n"

rows = list(csv.DictReader(io.StringIO("name,age\r\nalice,30\r\nbob,\r\n")))
assert rows[0]["name"] == "alice" and rows[0]["age"] == "30"
assert rows[1]["name"] == "bob" and rows[1]["age"] == ""
r = csv.DictReader(io.StringIO("name,age\r\nx,1\r\n"))
assert list(r) == [{"name": "x", "age": "1"}]
assert r.fieldnames == ["name", "age"]

r2 = csv.DictReader(io.StringIO("1,2\r\n"), fieldnames=["a", "b"])
assert list(r2) == [{"a": "1", "b": "2"}]

assert (255).to_bytes(2, "big") == b"\x00\xff"
assert (255).to_bytes(2, "little") == b"\xff\x00"
assert int.from_bytes(b"\x00\xff", "big") == 255
assert int.from_bytes(b"\x00\xff", "little") == 65280
assert int.from_bytes([1, 0], "big") == 256
try:
    (256).to_bytes(1)
    assert False, "expected OverflowError"
except OverflowError:
    pass
try:
    int.from_bytes("ab", "big")
    assert False, "expected TypeError"
except TypeError:
    pass

print("llm_usability_test passed")
