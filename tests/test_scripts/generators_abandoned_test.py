# A generator a loop leaves early is closed, so its cleanup runs.
# Behaves the same under CPython 3.
log = []

def numbers(tag):
    try:
        for i in range(10):
            yield i
    finally:
        log.append("closed " + tag)

# break out of a loop over a fresh generator
for n in numbers("break"):
    if n == 2:
        break
assert log == ["closed break"]

# return from inside the loop
def first_even(tag):
    for n in numbers(tag):
        if n and n % 2 == 0:
            return n
    return None
assert first_even("return") == 2
assert log[-1] == "closed return"

# an exception inside the loop body
try:
    for n in numbers("raise"):
        raise ValueError("stop")
except ValueError:
    pass
assert log[-1] == "closed raise"

# a loop that finishes runs the cleanup once, not twice
del log[:]
for n in numbers("full"):
    pass
assert log == ["closed full"]

# methods that are generators
class Tree:
    def __init__(self):
        self.closed = 0
    def walk(self):
        try:
            yield 1
            yield 2
        finally:
            self.closed += 1
t = Tree()
for v in t.walk():
    break
assert t.closed == 1

# a generator held in a variable is not closed by a loop leaving it
del log[:]
held = numbers("held")
for n in held:
    if n == 1:
        break
assert log == []
assert next(held) == 2
held.close()
assert log == ["closed held"]

# with blocks inside generators
class Res:
    def __init__(self):
        self.open = False
    def __enter__(self):
        self.open = True
        return self
    def __exit__(self, a, b, c):
        self.open = False
        return False
res = Res()
def using():
    with res:
        yield 1
        yield 2
for v in using():
    assert res.open
    break
assert res.open == False

# nested loops over generators
del log[:]
for a in numbers("outer"):
    for b in numbers("inner"):
        break
    break
assert log == ["closed inner", "closed outer"]
print("OK")
