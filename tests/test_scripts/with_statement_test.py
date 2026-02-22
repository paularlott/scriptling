# Tests for the 'with' statement / context managers (§1.4)

# ── Basic __enter__ / __exit__ ────────────────────────────────────────────────

class ManagedResource:
    def __init__(self, name):
        self.name = name
        self.entered = False
        self.exited = False
        self.exc_val = None

    def __enter__(self):
        self.entered = True
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.exited = True
        self.exc_val = exc_val
        return False  # don't suppress exceptions

r = ManagedResource("test")
with r:
    pass

assert r.entered == True, "expected __enter__ to be called"
assert r.exited == True, "expected __exit__ to be called"

# ── 'as' binding ──────────────────────────────────────────────────────────────

class CM:
    def __enter__(self):
        return 42

    def __exit__(self, exc_type, exc_val, exc_tb):
        return False

with CM() as val:
    result = val

assert result == 42, f"expected 42, got {result}"

# ── __exit__ called on exception ──────────────────────────────────────────────

class TrackingCM:
    def __init__(self):
        self.exited = False
        self.got_exc = False

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.exited = True
        if exc_val is not None:
            self.got_exc = True
        return False

tracker = TrackingCM()
caught = False
try:
    with tracker:
        raise ValueError("boom")
except:
    caught = True

assert tracker.exited == True, "__exit__ must be called even on exception"
assert tracker.got_exc == True, "__exit__ should receive exception info"
assert caught == True, "exception should propagate when __exit__ returns False"

# ── __exit__ suppresses exception when returning True ─────────────────────────

class SuppressCM:
    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        return True  # suppress

suppressed = False
with SuppressCM():
    raise ValueError("should be suppressed")
suppressed = True

assert suppressed == True, "exception should be suppressed when __exit__ returns True"

# ── __enter__ return value used as 'as' target ────────────────────────────────

class FactoryCM:
    def __enter__(self):
        return {"key": "value"}

    def __exit__(self, exc_type, exc_val, exc_tb):
        return False

with FactoryCM() as d:
    v = d["key"]

assert v == "value", f"expected 'value', got {v}"

# ── Nested with statements ────────────────────────────────────────────────────

class Counter:
    def __init__(self, name):
        self.name = name
        self.depth = 0

    def __enter__(self):
        self.depth = self.depth + 1
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        self.depth = self.depth - 1
        return False

outer = Counter("outer")
inner = Counter("inner")

with outer as o:
    with inner as i:
        inner_depth = i.depth
        outer_depth = o.depth

assert outer_depth == 1, f"outer depth inside = {outer_depth}"
assert inner_depth == 1, f"inner depth inside = {inner_depth}"
assert outer.depth == 0, f"outer depth after = {outer.depth}"
assert inner.depth == 0, f"inner depth after = {inner.depth}"

# ── Inheritance of __enter__ / __exit__ ───────────────────────────────────────

class BaseCM:
    def __enter__(self):
        return "base"

    def __exit__(self, exc_type, exc_val, exc_tb):
        return False

class DerivedCM(BaseCM):
    pass

with DerivedCM() as x:
    inherited_val = x

assert inherited_val == "base", f"expected 'base', got {inherited_val}"

True
