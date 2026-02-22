import contextlib

# suppress a matching exception type
with contextlib.suppress(ValueError):
    raise ValueError("ignored")

# suppress Exception catches all
with contextlib.suppress(Exception):
    raise Exception("also ignored")

# non-matching type is NOT suppressed
caught = False
try:
    with contextlib.suppress(ValueError):
        raise TypeError("not suppressed")
except TypeError:
    caught = True
assert caught, "TypeError should not have been suppressed"

# no exception — body runs normally
result = 0
with contextlib.suppress(ValueError):
    result = 42
assert result == 42, f"body should run: {result}"

# suppress with no types suppresses everything
with contextlib.suppress():
    raise Exception("suppressed by bare suppress")

# from import
from contextlib import suppress
with suppress(ValueError):
    raise ValueError("from-import works")

True
