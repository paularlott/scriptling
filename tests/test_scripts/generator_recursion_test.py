# Generators and the recursion limit behave as in Python 3: this script also
# runs unchanged under CPython 3.14.

def count(n):
    for i in range(n):
        yield i

assert list(count(3)) == [0, 1, 2]
it = count(2)
assert next(it) == 0 and next(it) == 1
assert next(it, "done") == "done"

def naturals():
    n = 0
    while True:
        yield n
        n += 1

lazy = naturals()
assert [next(lazy) for _ in range(5)] == [0, 1, 2, 3, 4]

# A pipeline of generators, each consuming the one before: 300 deep, well
# within the limit, works as before.
def add_one(src):
    for v in src:
        yield v + 1

pipe = count(3)
for _ in range(300):
    pipe = add_one(pipe)
assert list(pipe) == [300, 301, 302]

# A recursive generator within the limit: resuming the outer one resumes
# 50 nested ones.
def upto(n):
    for v in (upto(n - 1) if n > 0 else iter([])):
        yield v
    yield n

assert list(upto(50)) == list(range(51))

# A generator whose body resumes another without end recursed in Go until
# the host crashed; it now raises RecursionError, which a script can catch.
def runaway(n):
    yield next(runaway(n + 1))

g = runaway(0)
try:
    next(g)
    raise AssertionError("no RecursionError")
except RecursionError:
    pass
# The generator that raised is finished.
assert next(g, "finished") == "finished"

# Ordinary recursion raises the same, catchable as RecursionError,
# RuntimeError and Exception.
def deep(n):
    return deep(n + 1)

for exc in (RecursionError, RuntimeError, Exception):
    try:
        deep(0)
        raise AssertionError("no error")
    except exc as e:
        assert type(e).__name__ == "RecursionError"

# The limit is per call chain: recursion works again afterwards.
def depth(n):
    return 0 if n == 0 else 1 + depth(n - 1)
assert depth(200) == 200
