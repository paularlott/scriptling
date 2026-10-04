# Test the sentinel() builtin (PEP 661, Python 3.15)
#
# Each sentinel() call returns a new object that is equal only to itself.
# The intended check is identity: value is MISSING.

MISSING = sentinel("MISSING")
OTHER = sentinel("MISSING")  # same name, still a distinct sentinel

# repr defaults to the name; repr= overrides it
assert repr(MISSING) == "MISSING"
assert str(MISSING) == "MISSING"
assert f"{MISSING}" == "MISSING"
NOT_GIVEN = sentinel("not_given", repr="<NOT_GIVEN>")
assert repr(NOT_GIVEN) == "<NOT_GIVEN>"
assert str(NOT_GIVEN) == "<NOT_GIVEN>"

# Identity: a sentinel is itself, and nothing else
assert MISSING is MISSING
assert not (MISSING is OTHER)
assert MISSING is not OTHER
assert MISSING is not None
assert MISSING is not True

# == agrees with identity: true only against the same sentinel
assert MISSING == MISSING
assert not (MISSING == OTHER)
assert MISSING != OTHER
assert not (MISSING == None)
assert MISSING != None
assert not (MISSING == "MISSING")

# Each call creates a distinct sentinel, even with the same name
assert MISSING is not OTHER
assert MISSING != OTHER

# Sentinels are truthy
assert bool(MISSING)
assert not (not MISSING)
if MISSING:
    branch_taken = True
else:
    branch_taken = False
assert branch_taken

# __name__ attribute and getattr()/hasattr()
assert MISSING.__name__ == "MISSING"
assert getattr(MISSING, "__name__") == "MISSING"
assert getattr(MISSING, "nope", "fallback") == "fallback"
assert not hasattr(MISSING, "nope")

# The motivating use: a default-argument marker that None cannot collide with
def fetch(key, default=MISSING):
    if default is MISSING:
        return "no default"
    return f"default {default}"

assert fetch("k") == "no default"
assert fetch("k", 0) == "default 0"
assert fetch("k", "") == "default "
assert fetch("k", None) == "default None"

# Membership by identity: lists and sets
assert MISSING in [MISSING, 1, 2]
assert OTHER not in [MISSING, 1, 2]
group = {MISSING, 1}
assert MISSING in group
assert OTHER not in group

# Dict keys work by identity
d = {}
d[MISSING] = 1
assert d[MISSING] == 1
key_error = False
try:
    d[OTHER]
except KeyError:
    key_error = True
assert key_error

# hash() is defined and stable for a given sentinel
assert isinstance(hash(MISSING), int)
assert hash(MISSING) == hash(MISSING)

# Non-string names raise TypeError
def raises_type_error(fn):
    try:
        fn()
        return False
    except TypeError:
        return True

assert raises_type_error(lambda: sentinel(42))
assert raises_type_error(lambda: sentinel("X", repr=42))

# Ordering comparisons are not supported
assert raises_type_error(lambda: MISSING < OTHER)
assert raises_type_error(lambda: MISSING >= MISSING)

# Sentinels are not callable
assert raises_type_error(lambda: MISSING())

# Other unsupported operations raise AttributeError / TypeError, as in Python
def raises_attribute_error(fn):
    try:
        fn()
        return False
    except AttributeError:
        return True

assert raises_attribute_error(lambda: MISSING.nope)
assert raises_attribute_error(lambda: getattr(MISSING, "nope"))

assert raises_type_error(lambda: len(MISSING))
assert raises_type_error(lambda: MISSING[0])
assert raises_type_error(lambda: MISSING + 1)
assert raises_type_error(lambda: [x for x in MISSING])
assert raises_type_error(lambda: MISSING in MISSING)

# Inside containers, repr uses the sentinel's repr, like Python
assert repr([MISSING, 1]) == "[MISSING, 1]"
assert repr((MISSING,)) == "(MISSING,)"
assert str({1: MISSING}) == "{1: MISSING}"

True
