# Integer values must not change once a script can reach them, whichever
# arithmetic path produced them. Covers the unboxed integer fast paths
# (compiled int operands, fused augmented assignment) and the small-integer
# cache boundaries (-5 and 10000), plus overflow at the 64-bit limits.

# Values around the cache boundaries: below, at and above, both signs.
edges = [-1000, -7, -6, -5, -4, -1, 0, 1, 255, 256, 257, 1023, 1024, 1025,
         9999, 10000, 10001, 65535, 65536, 65537, 1 << 40]
for e in edges:
    a = e
    b = a + 1
    c = a - 1
    assert b - a == 1
    assert a - c == 1
    assert b == e + 1 and c == e - 1
    assert a == e
    d = a * 2 - a
    assert d == e
    t = a
    t += 1
    assert t == e + 1 and a == e
    t -= 2
    assert t == e - 1 and a == e
    t *= 3
    assert t == (e - 1) * 3

# Loop variables captured by closures, lists, dicts and sets, checked after the
# loop has moved on.
fns = []
kept = []
as_keys = {}
members = set()
for i in range(9990, 10020):
    fns.append(lambda i=i: i)
    kept.append(i)
    as_keys[i] = i * 2
    members.add(i)
    j = i + 0
    kept.append(j)
for n, f in enumerate(fns):
    assert f() == 9990 + n
assert kept[0] == 9990 and kept[-1] == 10019 and kept[-2] == 10019
assert kept == [v for i in range(9990, 10020) for v in (i, i)]
for i in range(9990, 10020):
    assert as_keys[i] == i * 2
    assert i in members
assert 10020 not in members and 9989 not in as_keys

# Late-binding closures see the final value, as in Python.
late = [lambda: k for k in range(10010, 10013)]
assert [f() for f in late] == [10012, 10012, 10012]

# While-loop counters advanced with +=, stored at every step.
seen = []
i = 9995
while i < 10010:
    seen.append(i)
    i += 1
assert seen == list(range(9995, 10010))
assert i == 10010

# Augmented assignment results stored, then the variable changed again.
t = 10000
snapshots = []
for step in range(5):
    t += step * 3
    snapshots.append(t)
    t -= 1
assert snapshots == [10000, 10002, 10007, 10015, 10026]
assert t == 10025

t = 123456789
saved = t
t += 1
assert saved == 123456789 and t == 123456790
t = t * 2 + 7
assert saved == 123456789 and t == 246913587

# Fused value expressions in augmented assignment.
acc = 0
for i in range(20000):
    acc += i * 2 - 1
assert acc == 2 * (19999 * 20000 // 2) - 20000

acc = 1000003
for i in range(1, 50):
    acc %= i + 7
    acc += i * i
assert acc == 2437

bits = 0
for i in range(40):
    bits |= 1 << i
    bits ^= i
assert bits == 1099511627775

# Integers as dict keys and set members across the cache boundary, mixing
# values produced by literals, arithmetic and loops.
d = {}
for k in [-6, -5, -4, 9999, 10000, 10001, 70000]:
    d[k] = str(k)
assert d[-3 - 3] == "-6"
assert d[-10 + 5] == "-5"
assert d[10000 - 1] == "9999"
assert d[5000 * 2] == "10000"
assert d[10000 + 1] == "10001"
assert d[69999 + 1] == "70000"
s = {10000 + i for i in range(-2, 3)}
assert s == {9998, 9999, 10000, 10001, 10002}
assert 10001 in s and (20002 // 2) in s

# Floor division and modulo keep Python semantics on the fast paths.
assert (-7) // 2 == -4 and (-7) % 2 == 1
assert 7 // -2 == -4 and 7 % -2 == -1
x = -10001
assert x // 3 == -3334 and x % 3 == 1
x = 10001
x //= -3
assert x == -3334
x = 10001
x %= -3
assert x == -1

# Errors still raised from the fast paths.
z = 0
try:
    q = 10 // z
    assert False
except ZeroDivisionError:
    pass
try:
    q = 10
    q %= z
    assert False
except ZeroDivisionError:
    pass

# Overflow at the 64-bit limits: scriptling ints are 64-bit and raise
# OverflowError where Python would promote to a big integer, so the checks
# only run when the result does not fit.
big = 9223372036854775807
small = -9223372036854775807 - 1


def overflows(fn):
    try:
        r = fn()
    except OverflowError:
        return True
    return r > big or r < small


def aug_add(v, n):
    v += n
    return v


def aug_sub(v, n):
    v -= n
    return v


def aug_mul(v, n):
    v *= n
    return v


def aug_expr(v, n):
    v += n * 2
    return v


assert overflows(lambda: big + 1)
assert overflows(lambda: small - 1)
assert overflows(lambda: big * 2)
assert overflows(lambda: aug_add(big, 1))
assert overflows(lambda: aug_sub(small, 1))
assert overflows(lambda: aug_mul(big, 3))
assert overflows(lambda: aug_expr(big, 1))
assert aug_add(big - 1, 1) == big
assert aug_sub(small + 1, 1) == small

# Mixed int, float and bool arithmetic keeps Python's result types.
n = 10001
f = n + 0.5
assert type(f) == float and f == 10001.5
m = n
m += 0.5
assert type(m) == float and m == 10001.5
b = True
b += 10000
assert type(b) == int and b == 10001
c = n + True
assert type(c) == int and c == 10002
c = n * False
assert type(c) == int and c == 0
e = n / 1
assert type(e) == float and e == 10001.0
g = n
g //= 2.0
assert type(g) == float and g == 5000.0
h = 3
h += n * 2
assert type(h) == int and h == 20005

# A name rebound from int to another type mid-loop leaves the fast path.
v = 10000
for step in range(3):
    if step == 1:
        v = 2.5
    v += 1
assert v == 4.5

print("OK")
