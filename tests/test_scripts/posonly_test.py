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

assert h(1, z=3) == (1, 2, [], 3, {})
assert h(1, 9, 8, 7, z=3, w=4) == (1, 9, [8, 7], 3, {"w": 4})
assert raises_type_error(lambda: h(x=1, z=3))

# **kwargs does not rescue a positional-only name: k(1, a=2) is a TypeError
# even though **kw could collect it, exactly as in Python
def k(a, /, **kw):
    return kw

assert k(1, b=2) == {"b": 2}
assert raises_type_error(lambda: k(1, a=2))
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
