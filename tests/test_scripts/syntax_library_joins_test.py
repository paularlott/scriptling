# Pieces that span the generator, parser and builtin work.
# Behaves the same under CPython 3.

# the ellipsis literal
def stub(): ...
assert stub() is None
assert ... is Ellipsis
marker = ...
assert marker is Ellipsis and marker is not None

# a generator's return value survives next()
def gen():
    yield 1
    return "done"
g = gen()
assert next(g) == 1
try:
    next(g)
    value = "no stop"
except StopIteration as e:
    value = e.value
assert value == "done"
assert next(gen(), "default") == 1
g2 = gen()
next(g2)
assert next(g2, "fallback") == "fallback"

# object as a base, a sentinel, and __setattr__ through object
class Base(object):
    pass
MISSING = object()
assert isinstance(Base(), object) and MISSING is not object()
class Guard:
    def __setattr__(self, name, value):
        object.__setattr__(self, name, str(value))
gd = Guard()
gd.n = 5
assert gd.n == "5"

# contextmanager with setup and teardown around the body
from contextlib import contextmanager
events = []
@contextmanager
def session(name):
    events.append("open " + name)
    yield name.upper()
    events.append("close " + name)
with session("a") as s, session("b") as t:
    events.append(s + t)
assert events == ["open a", "open b", "AB", "close b", "close a"]

# match on what a generator delegates
def numbers():
    yield from [(1, 2), (3,), "x"]
out = []
for item in numbers():
    match item:
        case (a, b):
            out.append(a + b)
        case (a,):
            out.append(a)
        case _:
            out.append(None)
assert out == [3, 3, None]
print("OK")
