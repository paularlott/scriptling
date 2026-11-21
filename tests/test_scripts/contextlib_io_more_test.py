# contextlib: suppress matches through the exception hierarchy;
# nullcontext, closing.
import contextlib

# suppress matches as except does
with contextlib.suppress(LookupError):
    {}["x"]
with contextlib.suppress(ArithmeticError):
    1 / 0
with contextlib.suppress(TypeError, ValueError):
    int("x")

class E(ValueError):
    pass

with contextlib.suppress(ValueError):
    raise E("x")
try:
    with contextlib.suppress(KeyError):
        1 / 0
    assert False
except ZeroDivisionError:
    pass

with contextlib.nullcontext(5) as v:
    assert v == 5
with contextlib.nullcontext() as v:
    assert v is None

closed = []

class Thing:
    def close(self):
        closed.append(True)

with contextlib.closing(Thing()) as t:
    assert type(t).__name__ == "Thing"
assert closed == [True]
print("OK")
