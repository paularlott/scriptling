import contextlib
from contextlib import contextmanager, nullcontext, closing

log = []


@contextmanager
def tag(name):
    log.append("<" + name + ">")
    yield name.upper()
    log.append("</" + name + ">")


# the yielded value is the as target; code after the yield runs on exit
with tag("b") as t:
    log.append(t)
assert log == ["<b>", "B", "</b>"], log

# an exception in the body propagates; the code after the yield is skipped
# (the generator does not catch it)
log.clear()
try:
    with tag("i"):
        raise ValueError("boom")
except ValueError as e:
    assert str(e) == "boom"
assert log == ["<i>"], log


# arguments and keyword arguments reach the generator function
@contextlib.contextmanager
def added(a, b=2):
    yield a + b


with added(1, b=5) as total:
    assert total == 6
assert tag.__name__ == "tag"


# setup in a loop before the yield
@contextmanager
def summed(values):
    total = 0
    for v in values:
        total += v
    yield total
    log.append("done")


log.clear()
with summed([1, 2, 3]) as s:
    assert s == 6
assert log == ["done"]


# a generator that never yields, or yields twice, is a RuntimeError
@contextmanager
def never():
    for _ in []:
        yield 1


@contextmanager
def twice():
    yield 1
    yield 2


for factory, text in ((never, "didn't yield"), (twice, "didn't stop")):
    try:
        with factory():
            pass
        assert False, "expected RuntimeError"
    except RuntimeError as e:
        assert text in str(e), str(e)

# nullcontext and closing
with nullcontext(5) as v:
    assert v == 5
with nullcontext() as v:
    assert v is None


class Resource:
    closed = False

    def close(self):
        self.closed = True


r = Resource()
with closing(r) as got:
    assert got is r and not r.closed
assert r.closed

print("OK")
