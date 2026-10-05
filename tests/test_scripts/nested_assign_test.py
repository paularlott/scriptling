# Destructuring assignment targets: parenthesized, bracketed, nested, and
# mixed with index/attribute targets, matching CPython semantics.
# (For-loop tuple targets are covered elsewhere; a regression pin is included.)

# --- Parenthesized flat targets -------------------------------------------
(a, b) = (1, 2)
assert a == 1 and b == 2

(a, b) = [10, 20]
assert a == 10 and b == 20

(a, b) = "xy"
assert a == "x" and b == "y"

(a, b) = {"k1": 1, "k2": 2}
assert sorted([a, b]) == ["k1", "k2"]

(a, b) = range(5, 7)
assert a == 5 and b == 6

# Single-element and empty groups
(c,) = (9,)
assert c == 9
() = ()
assert True

# Swap idiom
x, y = 1, 2
(x, y) = (y, x)
assert x == 2 and y == 1

# --- Nested targets --------------------------------------------------------
(a, (b, c)) = (1, (2, 3))
assert (a, b, c) == (1, 2, 3)

[a, [b, c]] = [1, [2, 3]]
assert (a, b, c) == (1, 2, 3)

(a, (b, c)) = 1, (2, 3)  # unparenthesized target list with a group
assert (a, b, c) == (1, 2, 3)

(a, [b, (c, d)]) = (1, [2, (3, 4)])
assert (a, b, c, d) == (1, 2, 3, 4)

((a, b), c) = (1, 2), 3  # leading group, unparenthesized rest
assert (a, b, c) == (1, 2, 3)

((a, b), (c, d)) = ((1, 2), (3, 4))
assert (a, b, c, d) == (1, 2, 3, 4)

# Nested unpacking from an iterable source: each level matches lengths exactly
(k, (v1, v2)) = ["a", "bc"]
assert (k, v1, v2) == ("a", "b", "c")

try:
    (k, (v1, v2)) = "abc"  # outer level sees 3 elements for 2 targets
    assert False, "expected ValueError"
except ValueError:
    pass

# Deep nesting
(a, (b, (c, (d, e)))) = (1, (2, (3, (4, 5))))
assert (a, b, c, d, e) == (1, 2, 3, 4, 5)

# --- Index and attribute targets ------------------------------------------
lst = [0, 0]
lst[0], y = 5, 6
assert lst == [5, 0] and y == 6

lst = [0, 0]
a, lst[1] = 7, 8
assert a == 7 and lst == [0, 8]

d = {}
d["k"], (b, c) = 5, (6, 7)
assert d["k"] == 5 and (b, c) == (6, 7)


class Box:
    pass


o = Box()
o.x, y = 7, 8
assert o.x == 7 and y == 8

o = Box()
a, o.x = 7, 8
assert a == 7 and o.x == 8

o = Box()
o.x, (b, c) = 7, (8, 9)
assert o.x == 7 and (b, c) == (8, 9)

# Slice target mixed into a group
sl = [1, 2, 3, 4, 5]
sl[1:3], tail = ["a", "b"], [9]
assert sl == [1, "a", "b", 4, 5] and tail == [9]

# Duplicate name: later element wins, as in CPython
(z, z) = (1, 2)
assert z == 2

# --- Starred flat form still intact ---------------------------------------
a, *mid, c = [1, 2, 3, 4]
assert a == 1 and mid == [2, 3] and c == 4

*a, b = [1, 2]
assert a == [1] and b == 2

# --- Error paths (CPython messages and types) ------------------------------
try:
    (a, b) = [1, 2, 3]
    assert False, "expected ValueError"
except ValueError as e:
    assert str(e) == "too many values to unpack (expected 2, got 3)"

try:
    (a, b) = [1]
    assert False, "expected ValueError"
except ValueError as e:
    assert str(e) == "not enough values to unpack (expected 2, got 1)"

try:
    (a, b) = 5
    assert False, "expected TypeError"
except TypeError:
    pass

try:
    (a, (b, c)) = (1, [2, 3, 4])  # inner level mismatch
    assert False, "expected ValueError"
except ValueError as e:
    assert "too many values to unpack" in str(e)

# Flat unparenthesized form reports the same errors as the grouped form
try:
    a, b = [1, 2, 3]
    assert False, "expected ValueError"
except ValueError as e:
    assert str(e) == "too many values to unpack (expected 2, got 3)"

try:
    a, b = [1]
    assert False, "expected ValueError"
except ValueError as e:
    assert str(e) == "not enough values to unpack (expected 2, got 1)"

try:
    a, b = 5
    assert False, "expected TypeError"
except TypeError as e:
    assert "cannot unpack non-iterable" in str(e)

try:
    a, *b, c = [1]
    assert False, "expected ValueError"
except ValueError as e:
    assert str(e) == "not enough values to unpack (expected at least 2, got 1)"

# Value side can be any expression yielding an iterable
(a, b) = (1, (2, 3), 4)[0:2]
assert a == 1 and b == (2, 3)

# --- Chained assignment with any target shape -------------------------------
# Python: (target_list "=")+ value — every target gets the value, evaluated once.
(a, b) = c = (1, 2)
assert a == 1 and b == 2 and c == (1, 2)

a = (b, c) = (1, 2)
assert a == (1, 2) and b == 1 and c == 2

a, b = c, d = 1, 2
assert (a, b, c, d) == (1, 2, 1, 2)

x = [0]
x[0] = y = 5
assert x == [5] and y == 5

p = q = r = 9
assert (p, q, r) == (9, 9, 9)

(a, b) = c = d = (3, 4)
assert (a, b, c, d) == (3, 4, (3, 4), (3, 4))

n = (o, (s2, t)) = (1, (2, 3))
assert n == (1, (2, 3)) and (o, s2, t) == (1, 2, 3)

dd = {}
dd["k"] = e = 4
assert dd == {"k": 4} and e == 4

x = [0]
x[0] = (yy, zz) = (5, 6)
assert x == [(5, 6)] and yy == 5 and zz == 6

# The shared value is evaluated exactly once
cell = {"n": 0}
def once_fn():
    cell["n"] = cell["n"] + 1
    return cell["n"] * 10

u = v = once_fn()
assert (u, v, cell["n"]) == (10, 10, 1)

# Mutable value is shared by every target, as in Python
s1 = s2b = [7]
s1.append(8)
assert s2b == [7, 8]

# --- For-loop tuple targets (regression pin) --------------------------------
pairs = {"ab": (1, 2)}
for k, (v1, v2) in pairs.items():
    assert k == "ab" and v1 == 1 and v2 == 2

for (p, q), r in [((1, 2), 3), ((4, 5), 6)]:
    assert q - p == 1

# --- Chained and augmented forms unaffected --------------------------------
m = n = 3
assert m == 3 and n == 3

i = 1
i += 2
assert i == 3

print("nested_assign_test passed")
