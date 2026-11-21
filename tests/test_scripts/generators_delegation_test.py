# yield from, generator send(), return values, tuple yields, close and throw

# Delegation to every kind of iterable
def from_list():
    yield from [1, 2, 3]

def from_string():
    yield from "ab"

def from_range():
    yield from range(3)

def inner():
    yield 10
    yield 20

def from_generator():
    yield 0
    yield from inner()
    yield 30

class Countdown:
    def __init__(self, n):
        self.n = n
    def __iter__(self):
        return iter(list(range(self.n, 0, -1)))

def from_instance():
    yield from Countdown(3)

assert list(from_list()) == [1, 2, 3]
assert list(from_string()) == ["a", "b"]
assert list(from_range()) == [0, 1, 2]
assert list(from_generator()) == [0, 10, 20, 30]
assert list(from_instance()) == [3, 2, 1]

# Empty delegation
def empty_inner():
    return
    yield 1

def from_empty():
    yield "start"
    yield from []
    yield from empty_inner()
    yield from ""
    yield "end"

assert list(from_empty()) == ["start", "end"]

# Three-deep nesting
def level3():
    yield 3
    return "l3"

def level2():
    r = yield from level3()
    yield r
    return "l2"

def level1():
    r = yield from level2()
    yield r

assert list(level1()) == [3, "l3", "l2"]

# Return value of a delegated generator
def summer(items):
    total = 0
    for x in items:
        yield x
        total += x
    return total

def report():
    total = yield from summer([1, 2, 3])
    yield "total=" + str(total)

assert list(report()) == [1, 2, 3, "total=6"]

# Return without value: yield from gives None
def no_value():
    yield 1
    return

def check_none():
    r = yield from no_value()
    yield r

assert list(check_none()) == [1, None]

# StopIteration.value carries the return value
def returns_five():
    yield 1
    return 5

g = returns_five()
assert next(g) == 1
try:
    g.__next__()
    assert False, "expected StopIteration"
except StopIteration as e:
    assert e.value == 5

# Exhausted generators keep raising StopIteration
try:
    next(g)
    assert False, "expected StopIteration"
except StopIteration:
    pass
try:
    g.__next__()
    assert False, "expected StopIteration"
except StopIteration as e:
    assert e.value is None
assert next(g, "default") == "default"

# yield as an expression with send()
def accumulator():
    total = 0
    while True:
        value = yield total
        if value is None:
            break
        total += value

acc = accumulator()
assert next(acc) == 0
assert acc.send(5) == 5
assert acc.send(10) == 15
try:
    acc.send(None)
    assert False, "expected StopIteration"
except StopIteration:
    pass

# Bare x = yield, and next() delivering None
def echo():
    received = []
    while len(received) < 3:
        x = yield
        received.append(x)
    yield received

e = echo()
next(e)
e.send("a")
next(e)
assert e.send("c") == ["a", None, "c"]

# Parenthesised yield as an assignment value
def paren_yield():
    x = (yield 1)
    yield x * 2

p = paren_yield()
assert next(p) == 1
assert p.send(21) == 42

# send(non-None) to a just-started generator is a TypeError
def simple():
    yield 1

s = simple()
try:
    s.send(1)
    assert False, "expected TypeError"
except TypeError:
    pass
assert next(s) == 1

# send passes through a delegating generator
def inner_echo():
    got = []
    while len(got) < 2:
        v = yield len(got)
        got.append(v)
    return got

def outer_echo():
    result = yield from inner_echo()
    yield result

oe = outer_echo()
assert next(oe) == 0
assert oe.send("x") == 1
assert oe.send("y") == ["x", "y"]

# Tuples after yield
def tuples(d):
    yield (d,)
    yield d, 2
    yield (d, 3)
    yield [d]

assert list(tuples(1)) == [(1,), (1, 2), (1, 3), [1]]

# Tuple targets for a sent value
def pairs():
    a, b = yield "ready"
    yield a + b

pr = pairs()
assert next(pr) == "ready"
assert pr.send((3, 4)) == 7

# Generators in comprehensions
def squares(n):
    for i in range(n):
        yield i * i

def chain(*iterables):
    for it in iterables:
        yield from it

assert [x for x in chain(squares(3), [9])] == [0, 1, 4, 9]
assert {x for x in chain("aba")} == {"a", "b"}
assert sum(x for x in chain(squares(4))) == 14
assert dict((k, v) for k, v in chain([("a", 1)], [("b", 2)])) == {"a": 1, "b": 2}

# Yields inside if/elif/else and nested loops
def flatten(items):
    for item in items:
        if isinstance(item, list):
            yield from flatten(item)
        else:
            yield item

assert list(flatten([1, [2, [3, [4]], 5], [], 6])) == [1, 2, 3, 4, 5, 6]

def classify(nums):
    for n in nums:
        if n < 0:
            yield "neg"
        elif n == 0:
            yield "zero"
        else:
            for _ in range(n):
                yield "pos"

assert list(classify([-1, 0, 2])) == ["neg", "zero", "pos", "pos"]

def grid(w, h):
    for y in range(h):
        for x in range(w):
            if x == y:
                continue
            yield (x, y)
            if x + y > 2:
                break

assert list(grid(3, 3)) == [(1, 0), (2, 0), (0, 1), (2, 1), (0, 2), (1, 2)]

# Loop else clauses with yields
def search(items, target):
    for item in items:
        if item == target:
            yield "found"
            break
        yield item
    else:
        yield "missing"

assert list(search([1, 2, 3], 2)) == [1, "found"]
assert list(search([1, 3], 2)) == [1, 3, "missing"]

def countdown(n):
    while n > 0:
        yield n
        n -= 1
    else:
        yield "liftoff"

assert list(countdown(2)) == [2, 1, "liftoff"]

# Exceptions inside a delegated generator propagate with their type
def failing():
    yield 1
    raise ValueError("boom")

def delegating_failure():
    yield from failing()
    yield "unreachable"

caught = None
results = []
try:
    for v in delegating_failure():
        results.append(v)
except ValueError as err:
    caught = str(err)
assert results == [1]
assert caught == "boom"

def key_error_inner():
    yield {}["missing"]

def key_error_outer():
    yield from key_error_inner()

try:
    list(key_error_outer())
    assert False, "expected KeyError"
except KeyError:
    pass

# A failing iterable expression in yield from
def bad_source():
    yield from 5

try:
    list(bad_source())
    assert False, "expected TypeError"
except TypeError:
    pass

# Exception from a generator ends it
def raises_then():
    yield 1
    raise RuntimeError("stop")

rt = raises_then()
next(rt)
try:
    next(rt)
except RuntimeError:
    pass
try:
    next(rt)
    assert False, "expected StopIteration"
except StopIteration:
    pass

# close() ends a generator and the one it delegates to
def closable_inner():
    yield 1
    yield 2

def closable_outer(inner_gen):
    yield from inner_gen

ig = closable_inner()
cg = closable_outer(ig)
assert next(cg) == 1
cg.close()
assert list(cg) == []
assert list(ig) == []

# throw() raises at the paused yield and ends the generator
def throwable():
    yield 1
    yield 2

t = throwable()
next(t)
try:
    t.throw(ValueError("thrown"))
    assert False, "expected ValueError"
except ValueError as err:
    assert str(err) == "thrown"
assert list(t) == []

t2 = throwable()
try:
    t2.throw(KeyError)
    assert False, "expected KeyError"
except KeyError:
    pass

# return inside a loop ends the generator with a value
def first_even(nums):
    for n in nums:
        if n % 2 == 0:
            return n
        yield n
    return None

def find_even():
    r = yield from first_even([1, 3, 4, 5])
    yield ("even", r)

assert list(find_even()) == [1, 3, ("even", 4)]

# A captured local from yield is a function local
def local_binding():
    x = yield 1
    y = x
    yield y

lb = local_binding()
next(lb)
assert lb.send(7) == 7

print("OK")
