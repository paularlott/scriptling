# Test operator precedence matches Python: comparisons vs bitwise vs
# and/or/not, and the power/unary interaction.

# Bitwise ops bind tighter than comparisons (was: == bound tighter than &)
assert 3 & 1 == 1            # (3 & 1) == 1
assert 5 & 3 == 1
assert 1 | 2 == 3
assert 6 ^ 3 == 5
assert not (1 | 2 == 2)      # (1 | 2) == 2 is 3 == 2
assert repr(3 & 1 == 1) == "True"

# Comparisons bind tighter than and/or
assert 1 == 1 and 2 == 2
assert 1 == 2 or 3 == 3
assert not (1 == 2 and 3 == 3)

# Bitwise binds tighter than and/or (Python's order, not C's)
assert (1 | 2) and True
assert 4 & 2 == 0            # (4 & 2) == 0

# `not` is looser than comparisons: not a == b is not (a == b)
assert not 1 == 2
assert not 1 in [2, 3]
assert not True == False
assert (not 3) == False      # but `not` binds tighter than == for its own operand
assert not 0 and True        # (not 0) and True

# Chained comparisons keep Python semantics
assert 1 < 2 < 3
assert 1 < 2 == 2
assert not (1 < 2 == True)   # 1 < 2 and 2 == True → False
assert 3 > 2 >= 2

# Shifts bind tighter than &
assert 1 << 2 & 4 == 4       # ((1 << 2) & 4) == 4

# ** is right-associative and tighter than unary minus
assert -2 ** 2 == -4         # -(2 ** 2)
assert 2 ** 3 ** 2 == 512    # 2 ** (3 ** 2)
assert 2 ** -1 == 0.5
assert -2 * 3 == -6          # (-2) * 3
assert (2 * -3) == -6
assert -2 + 3 == 1           # (-2) + 3

# Arithmetic and the rest are unaffected
assert 1 + 2 * 3 == 7
assert (1 + 2) * 3 == 9
assert 10 - 4 - 3 == 3       # left-associative
assert 2 ** 10 == 1024

# Set algebra no longer needs parens around == comparisons
fs = frozenset([1, 2])
assert fs & {2} == frozenset([2])
assert fs | {3} == frozenset([1, 2, 3])
assert fs - {2} == frozenset([1])
assert fs ^ {2} == frozenset([1])
s = {1, 2}
assert s - {1} == {2}
assert s | {3} == {1, 2, 3}

True

# Unary minus and ~ keep working and raise Python's TypeError on bad
# operand types (catchable, not fatal)
assert --5 == 5
assert -5 == -5
assert ~5 == -6
assert ~~5 == 5

def _raises_type_error(fn):
    try:
        fn()
        return False
    except TypeError:
        return True

assert _raises_type_error(lambda: -("a" * 2))
assert _raises_type_error(lambda: -[1, 2])
assert _raises_type_error(lambda: -None)
assert _raises_type_error(lambda: ~"x")
assert _raises_type_error(lambda: ~2.5)

# bool is an int: -True is -1 and ~True is -2, as in Python
assert -True == -1
assert -False == 0
assert ~True == -2
