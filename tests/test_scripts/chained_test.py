# Test chained comparisons: Python semantics — each operand evaluates at
# most once, left to right, and the chain stops at the first false link
# (later operands are never evaluated).

# Values (kept from the precedence audit, now exercising the chain node)
assert 1 < 2 < 3
assert not (1 < 2 > 3)
assert 3 > 2 >= 2 < 4
assert 1 != 2 != 3
assert not (1 in [1] == True)  # chains: (1 in [1]) and ([1] == True) → False
assert not (0 == 1 in [1])
assert "a" in "abc" in "xabcy"

# Each operand evaluates exactly once
calls = []

def noisy(v):
    calls.append(v)
    return v

assert 1 < noisy(2) < 3
assert calls == [2]

calls.clear()
assert not (3 < noisy(2) < 1)
assert calls == [2]          # middle evaluated once, last skipped (short-circuit)

calls.clear()
assert 1 < noisy(2) < noisy(3) < noisy(4)
assert calls == [2, 3, 4]    # every operand exactly once

# A false first link skips the LAST operand, not the middle: Python
# evaluates a and b, compares, and only then decides whether c is needed
calls.clear()
assert not (5 < noisy(2) < 1)
assert calls == [2]

def boom():
    raise ValueError("must not be evaluated")

assert not (5 < 2 < boom())  # short-circuit: boom never runs

# Middle value carries forward across mixed operators
assert 3 > 2 == 2
assert 1 <= 2 <= 2 <= 3

# in/is links with integer operands use the general path, not the int
# fast path: membership and identity still chain correctly
assert 1 is 1 in [1]
assert not (3 is 3 == True)
assert 1 == 1 in [1, 2]

# Dunder comparisons participate, middle instances evaluated once
dunder_calls = []

class Tracked:
    def __init__(self, n):
        self.n = n

    def __lt__(self, other):
        dunder_calls.append(self.n)
        return self.n < other.n

assert Tracked(1) < Tracked(2) < Tracked(3)
assert dunder_calls == [1, 2]

dunder_calls.clear()
assert not (Tracked(3) < Tracked(2) < Tracked(1))
assert dunder_calls == [3]  # first link false: second __lt__ never runs

# Walrus inside a chain binds through the new node
y = 0
assert 1 < (y := 2) < 3
assert y == 2

# Loop-guard idiom stays fast-path correct, including at the boundaries
for i in range(5):
    assert 0 <= i < 5
    assert not (0 <= i < 0)

assert 0 <= 0 < 1
assert not (0 <= 5 < 5)

True
