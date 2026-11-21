# Generators suspended inside try, with and match blocks: finally and
# __exit__ run on exhaustion, close() and throw(); except clauses see
# exceptions raised after resumption and thrown in; contextmanager throws
# the with body's exception into the generator.
from contextlib import contextmanager

log = []


# try/finally around a yield, normal exhaustion
def tf(n):
    try:
        for i in range(n):
            yield i
    finally:
        log.append("fin")


assert list(tf(3)) == [0, 1, 2]
assert log == ["fin"]

# consumer breaks early, then closes: finally runs once, at close()
log = []
g = tf(10)
for v in g:
    if v == 2:
        break
assert log == []
g.close()
assert log == ["fin"]
g.close()
assert log == ["fin"]
assert list(g) == []

# a generator is its own iterator
g = tf(2)
assert iter(g) is g
assert list(g) == [0, 1]

# explicit close() on a generator paused inside try
log = []
g = tf(5)
assert next(g) == 0
g.close()
assert log == ["fin"]
try:
    next(g)
    assert False, "closed generator resumed"
except StopIteration:
    pass

# close() on a generator never started runs nothing
log = []
g = tf(5)
g.close()
assert log == []


# throw() caught by an except around the yield; the generator continues
def catcher():
    while True:
        try:
            yield "ready"
        except ValueError as e:
            log.append("caught " + str(e))
            yield "handled"


log = []
g = catcher()
assert next(g) == "ready"
assert g.throw(ValueError("boom")) == "handled"
assert log == ["caught boom"]
assert next(g) == "ready"
assert g.throw(ValueError("again")) == "handled"
assert log == ["caught boom", "caught again"]
g.close()


# throw() not caught: propagates with its type, finally runs exactly once
def guarded():
    try:
        yield 1
        yield 2
    except KeyError:
        log.append("wrong handler")
    finally:
        log.append("fin")


log = []
g = guarded()
assert next(g) == 1
try:
    g.throw(TypeError("bad"))
    assert False, "throw did not propagate"
except TypeError as e:
    assert str(e) == "bad"
assert log == ["fin"]
g.close()
assert log == ["fin"]
assert list(g) == []


# exception raised after resumption reaches the except around the yield
def resumes():
    try:
        x = yield 1
        log.append("got " + str(x))
        raise ValueError("after")
    except ValueError as e:
        log.append("except " + str(e))
        yield 2
    log.append("end")


log = []
g = resumes()
assert next(g) == 1
assert g.send(7) == 2
assert list(g) == []
assert log == ["got 7", "except after", "end"]


# else and finally around yields, return value through finally
def with_else():
    try:
        yield "body"
    except ValueError:
        yield "except"
    else:
        yield "else"
    finally:
        log.append("fin")
    return "done"


log = []
assert list(with_else()) == ["body", "else"]
assert log == ["fin"]
log = []
g = with_else()
next(g)
assert g.throw(ValueError) == "except"
try:
    next(g)
    assert False
except StopIteration as e:
    assert e.value == "done"
assert log == ["fin"]


# nested try blocks: inner finally, outer except, outer finally, in order
def nested():
    try:
        try:
            yield 1
            yield 2
        finally:
            log.append("inner fin")
    except RuntimeError:
        log.append("outer except")
        yield 3
    finally:
        log.append("outer fin")


log = []
g = nested()
assert next(g) == 1
assert g.throw(RuntimeError("x")) == 3
assert log == ["inner fin", "outer except"]
assert list(g) == []
assert log == ["inner fin", "outer except", "outer fin"]

log = []
g = nested()
next(g)
next(g)
g.close()
assert log == ["inner fin", "outer fin"]


# break and return out of a try inside a loop run the finally
def loop_break():
    for i in range(5):
        try:
            if i == 2:
                break
            yield i
        finally:
            log.append("fin %d" % i)
    yield "after"


log = []
assert list(loop_break()) == [0, 1, "after"]
assert log == ["fin 0", "fin 1", "fin 2"]


def early_return():
    try:
        yield 1
        return
    finally:
        log.append("fin")


log = []
assert list(early_return()) == [1]
assert log == ["fin"]


# the generator catching GeneratorExit and cleaning up
def catches_exit():
    try:
        yield 1
    except GeneratorExit:
        log.append("exit seen")
        raise


log = []
g = catches_exit()
next(g)
g.close()
assert log == ["exit seen"]


# except Exception does not catch GeneratorExit
def broad():
    try:
        yield 1
    except Exception:
        log.append("wrong")
    finally:
        log.append("fin")


log = []
g = broad()
next(g)
g.close()
assert log == ["fin"]


# a generator that yields while being closed
def stubborn():
    try:
        yield 1
    except GeneratorExit:
        yield 2


g = stubborn()
next(g)
try:
    g.close()
    assert False, "close accepted a yield"
except RuntimeError:
    pass


# yield inside with: __exit__ runs on exhaustion, close() and throw()
class Res:
    def __init__(self, name):
        self.name = name

    def __enter__(self):
        log.append("enter " + self.name)
        return self

    def __exit__(self, exc_type, exc, tb):
        if exc_type is None:
            log.append("exit " + self.name)
        else:
            log.append("exit " + self.name + " " + type(exc).__name__)
        return False


def using(n):
    with Res("r") as r:
        for i in range(n):
            yield r.name + str(i)


log = []
assert list(using(2)) == ["r0", "r1"]
assert log == ["enter r", "exit r"]

log = []
g = using(5)
next(g)
g.close()
assert log == ["enter r", "exit r GeneratorExit"]

log = []
g = using(5)
next(g)
try:
    g.throw(KeyError("k"))
    assert False
except KeyError:
    pass
assert log == ["enter r", "exit r KeyError"]


# with that suppresses: the generator carries on after the with block
class Swallow:
    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc, tb):
        return exc_type is not None and issubclass(exc_type, ValueError)


def swallowing():
    with Swallow():
        yield 1
        raise ValueError("gone")
    yield 2


assert list(swallowing()) == [1, 2]


# yield inside match case bodies
def classify(items):
    for item in items:
        match item:
            case int(n) if n < 0:
                yield "neg"
            case int(n):
                yield "int"
                yield n
            case [a, b]:
                yield "pair"
            case _:
                yield "other"


assert list(classify([-1, 5, [1, 2], "x"])) == ["neg", "int", 5, "pair", "other"]


# the canonical contextmanager: try/finally around the yield
@contextmanager
def opened(name):
    log.append("open " + name)
    try:
        yield name.upper()
    finally:
        log.append("close " + name)


log = []
with opened("a") as v:
    assert v == "A"
    log.append("body")
assert log == ["open a", "body", "close a"]

# an exception in the body propagates after the cleanup
log = []
try:
    with opened("b"):
        raise ValueError("body failed")
    assert False
except ValueError as e:
    assert str(e) == "body failed"
assert log == ["open b", "close b"]


# the generator catching the exception suppresses it
@contextmanager
def ignoring(kind):
    try:
        yield
    except kind as e:
        log.append("ignored " + str(e))


log = []
with ignoring(ValueError):
    raise ValueError("v")
assert log == ["ignored v"]

# ... but only the kind it catches
log = []
try:
    with ignoring(ValueError):
        raise IndexError("i")
    assert False
except IndexError:
    pass
assert log == []


# nested context managers unwind innermost first
log = []
try:
    with opened("outer"):
        with opened("inner"):
            raise RuntimeError("deep")
except RuntimeError:
    pass
assert log == ["open outer", "open inner", "close inner", "close outer"]

log = []
with opened("x"), opened("y"):
    log.append("both")
assert log == ["open x", "open y", "both", "close y", "close x"]


# a generator expression consumed partially then discarded
data = [3, 1, 4, 1, 5, 9, 2, 6]
squares = (v * v for v in data)
for s in squares:
    if s > 10:
        break
assert s == 16
squares = None
first_big = [v for v in data if v > 4][0]
assert first_big == 5

print("OK")
