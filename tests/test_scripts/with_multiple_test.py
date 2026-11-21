# Several context managers in one with statement

log = []

class CM:
    def __init__(self, name, suppress=False, fail_enter=False, fail_exit=False):
        self.name = name
        self.suppress = suppress
        self.fail_enter = fail_enter
        self.fail_exit = fail_exit

    def __enter__(self):
        log.append("enter " + self.name)
        if self.fail_enter:
            raise RuntimeError("enter " + self.name)
        return self.name.upper()

    def __exit__(self, exc_type, exc_val, exc_tb):
        log.append("exit " + self.name + (" exc" if exc_type is not None else ""))
        if self.fail_exit:
            raise KeyError("exit " + self.name)
        return self.suppress

# Enter left to right, exit right to left
log = []
with CM("a") as x, CM("b") as y:
    log.append("body " + x + y)
assert log == ["enter a", "enter b", "body AB", "exit b", "exit a"]

# Parenthesised form
log = []
with (CM("a") as x, CM("b") as y):
    log.append("body " + x + y)
assert log == ["enter a", "enter b", "body AB", "exit b", "exit a"]

# Parenthesised, multi-line, trailing comma
log = []
with (
    CM("a") as x,
    CM("b") as y,
    CM("c") as z,
):
    log.append("body " + x + y + z)
assert log == ["enter a", "enter b", "enter c", "body ABC", "exit c", "exit b", "exit a"]

# Without targets, and mixed
log = []
with CM("a"), CM("b") as y:
    log.append("body " + y)
assert log == ["enter a", "enter b", "body B", "exit b", "exit a"]

log = []
with (CM("a"), CM("b")):
    log.append("body")
assert log == ["enter a", "enter b", "body", "exit b", "exit a"]

# A parenthesised single expression is still an ordinary with
log = []
with (CM("solo")) as s:
    log.append("body " + s)
assert log == ["enter solo", "body SOLO", "exit solo"]

def make(name):
    return CM(name)

class Factory:
    def build(self, name):
        return CM(name)

log = []
with (Factory()).build("f") as f:
    log.append(f)
assert log == ["enter f", "F", "exit f"]

# An exception passes through the inner __exit__ first; the outer one
# sees it too when the inner does not suppress
log = []
try:
    with CM("outer") as o, CM("inner") as i:
        raise ValueError("boom")
except ValueError:
    log.append("caught")
assert log == ["enter outer", "enter inner", "exit inner exc", "exit outer exc", "caught"]

# Suppressed by the inner manager: the outer one sees no exception
log = []
with CM("outer"), CM("inner", suppress=True):
    raise ValueError("boom")
assert log == ["enter outer", "enter inner", "exit inner exc", "exit outer"]

# Suppressed by the outer manager
log = []
with CM("outer", suppress=True), CM("inner"):
    raise ValueError("boom")
log.append("after")
assert log == ["enter outer", "enter inner", "exit inner exc", "exit outer exc", "after"]

# A failing __enter__ exits only the managers already entered
log = []
try:
    with CM("a"), CM("b", fail_enter=True), CM("c"):
        log.append("body")
except RuntimeError:
    log.append("caught")
assert log == ["enter a", "enter b", "exit a exc", "caught"]

# An exception from an inner __exit__ reaches the outer one
log = []
try:
    with CM("a"), CM("b", fail_exit=True):
        log.append("body")
except KeyError:
    log.append("caught")
assert log == ["enter a", "enter b", "body", "exit b", "exit a exc", "caught"]

# Return and break leave through every __exit__
def early():
    with CM("a") as a, CM("b") as b:
        return a + b
    return "unreachable"

log = []
assert early() == "AB"
assert log == ["enter a", "enter b", "exit b", "exit a"]

log = []
for n in range(3):
    with CM("x" + str(n)), CM("y" + str(n)):
        if n == 1:
            break
assert log == ["enter x0", "enter y0", "exit y0", "exit x0", "enter x1", "enter y1", "exit y1", "exit x1"]

# Targets inside a function are locals
def local_targets():
    with CM("p") as p, CM("q") as q:
        pass
    return p + q

log = []
assert local_targets() == "PQ"

# Later context expressions see earlier targets
class Holder:
    def __init__(self, value):
        self.value = value
    def __enter__(self):
        return self.value
    def __exit__(self, *args):
        return False

with Holder(3) as a, Holder(a * 2) as b:
    assert (a, b) == (3, 6)

print("OK")
