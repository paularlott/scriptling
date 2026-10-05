# Bare starred tuples, nested/for star targets, per-iteration unpacking of any
# iterable, and ;-separated statements in one-line blocks.

# --- bare starred tuples ---
t = (1, 2)
x = *t, 3
assert x == (1, 2, 3)

def f():
    return *t, 4
assert f() == (1, 2, 4)
assert (0, *t, *[9]) == (0, 1, 2, 9)
z = *t,
assert z == (1, 2)

# --- star targets: flat, nested, in for headers ---
*a, b = [1, 2, 3]
assert a == [1, 2] and b == 3
first, *rest = "abc"
assert first == "a" and rest == ["b", "c"]
p, (q, *r) = 1, (2, 3, 4)
assert (p, q, r) == (1, 2, [3, 4])
(a1, *b1), c1 = [1, 2, 3], 4
assert (a1, b1, c1) == (1, [2, 3], 4)
x2, (*y2, z2) = 0, "hey"
assert (x2, y2, z2) == (0, ["h", "e"], "y")
d = {}
(d["a"], *d["b"]) = (1, 2, 3)
assert d == {"a": 1, "b": [2, 3]}
try:
    (u, *v, w), w2 = [1], 2
    assert False, "expected ValueError"
except ValueError:
    pass

seen = []
for x3, *r3 in [(1, 2, 3), (4, 5), (6,)]:
    seen.append((x3, r3))
assert seen == [(1, [2, 3]), (4, [5]), (6, [])]
seen = []
for *a4, b4 in [(1, 2, 3)]:
    seen.append((a4, b4))
assert seen == [([1, 2], 3)]
seen = []
for i, (j, *k) in [(1, (2, 3, 4))]:
    seen.append((i, j, k))
assert seen == [(1, 2, [3, 4])]
seen = []
for h, *m, l in ["abcd", "xyz"]:
    seen.append((h, m, l))
assert seen == [("a", ["b", "c"], "d"), ("x", ["y"], "z")]
try:
    for a5, *b5, c5 in [(1,)]:
        pass
    assert False, "expected ValueError"
except ValueError:
    pass

# --- per-iteration unpacking works for any iterable, errors are catchable ---
pairs = []
for a6, b6 in ["xy", "zw"]:
    pairs.append(a6 + b6)
assert pairs == ["xy", "zw"]
for bad, exc in (([(1, 2, 3)], ValueError), ([(1,)], ValueError), ([5], TypeError)):
    try:
        for a7, b7 in bad:
            pass
        assert False, "expected an exception"
    except exc:
        pass

# --- ;-separated statements belong to a one-line block ---
def one(): v = 1; return v
assert one() == 1

class Z:
    def g(self): self.a = 1; return 2
zz = Z()
assert zz.g() == 2 and zz.a == 1

if True: aa = 1; bb = 2
assert (aa, bb) == (1, 2)

cc = 0
if False: cc = 1; dd = 2
assert cc == 0

for i in range(2): s = i; tt = s * 2
assert (s, tt) == (1, 2)

n = 0
while n < 3: n += 1; mm = n
assert (n, mm) == (3, 3)

try: raise ValueError("x"); cc = 99
except ValueError: caught = 1; cc = 5
assert caught == 1 and cc == 5

def trailing(): return 1;
assert trailing() == 1
if 1: uu = 1
else: uu = 2
assert uu == 1
