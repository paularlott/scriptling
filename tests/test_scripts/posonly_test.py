# Test positional-only parameters: the '/' marker (PEP 570)

# Basic form: everything before '/' is positional-only
def f(a, b, /, c):
    return (a, b, c)

assert f(1, 2, 3) == (1, 2, 3)
assert f(1, 2, c=3) == (1, 2, 3)

def raises_type_error(fn):
    try:
        fn()
        return False
    except TypeError:
        return True

# a and b cannot be passed by keyword
assert raises_type_error(lambda: f(1, 2, a=3))
assert raises_type_error(lambda: f(1, 2, b=3))
assert raises_type_error(lambda: f(a=1, b=2, c=3))

# Trailing slash: all parameters positional-only
def g(a, b, /):
    return a + b

assert g(1, 2) == 3
assert raises_type_error(lambda: g(a=1, b=2))

# Combined with defaults, *args and keyword-only parameters
def h(x, /, y=2, *args, z, **kw):
    return (x, y, args, z, kw)

assert h(1, z=3) == (1, 2, (), 3, {})
assert h(1, 9, 8, 7, z=3, w=4) == (1, 9, (8, 7), 3, {"w": 4})
assert raises_type_error(lambda: h(x=1, z=3))

# A bare * after / keeps later parameters keyword-only: positional calls
# must not fill them (a compiled fast path once bypassed this for
# default-less functions; the rejection is asserted in Go tests since
# scriptling arity errors are not catchable from scripts)
def kwo(a, /, *, b):
    return (a, b)

assert kwo(1, b=2) == (1, 2)

def kwo_plain(a, *, b):
    return (a, b)

assert kwo_plain(1, b=2) == (1, 2)

# **kwargs does not rescue a positional-only name: k(1, a=2) is a TypeError
# even though **kw could collect it, exactly as in Python
def k(a, /, **kw):
    return kw

assert k(1, b=2) == {"b": 2}
# A positional-only name used as a keyword lands in **kw, as in Python.
assert k(1, a=2) == {"a": 2}
assert raises_type_error(lambda: k(a=1))

# Lambdas support the marker too
add = lambda a, /, b: a + b
assert add(1, 2) == 3
assert add(1, b=2) == 3
assert raises_type_error(lambda: add(a=1, b=2))

# Class methods share the same parameter parser
class Box:
    def __init__(self, size, /, label=""):
        self.size = size
        self.label = label

b = Box(3, label="big")
assert b.size == 3 and b.label == "big"
assert raises_type_error(lambda: Box(size=3))

# Malformed placements are parse-level errors in Go tests; functions without
# the marker are unaffected
def plain(a, b):
    return a - b

assert plain(b=2, a=5) == 3

True

# Keyword-only defaults are filled even when no keywords are passed and the
# positional count reaches the parameter count (a bug: x leaked outer scope).
def kw_fill(a, b, *args, x=5, y=6):
    return (a, b, args, x, y)

assert kw_fill(1, 2, 3, 4) == (1, 2, (3, 4), 5, 6)
assert kw_fill(1, 2) == (1, 2, (), 5, 6)

x = 111
def kw_no_leak(*args, x=5):
    return x
assert kw_no_leak(1, 2, 3) == 5

def bare_star_default(a, *, b=2):
    return (a, b)
assert bare_star_default(1) == (1, 2)

def required_kwonly(a, *, b):
    return b
try:
    required_kwonly(1)
    assert False, "expected TypeError"
except TypeError as e:
    assert "required keyword-only argument" in str(e)

def all_params(a, *args, k=7, **kw):
    return (a, args, k, kw)
assert all_params(1, 2, 3) == (1, (2, 3), 7, {})
