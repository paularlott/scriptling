# Regression test for the pooled argument-buffer free-list (object.AcquireArgs).
# A variadic function forces the slice-based call path (the 1/2/3-arg fast paths
# don't apply), so each capture() call borrows a buffer from the per-root pool.
# Capturing *args across many calls and checking every snapshot afterwards
# catches buffer-reuse corruption: if the backing array were shared/reused while
# a captured varargs still referenced it, earlier snapshots would mutate to hold
# later values.

failures = 0

# (1) Capture *args across many calls. The returned list is a copy (varargs
#     binding copies), so every snapshot must keep its own values even though the
#     underlying call buffers are reused.
def capture(*args):
    return args

snapshots = []
for i in range(300):
    snapshots.append(capture(i, i * 10, i * 100))

for i, s in enumerate(snapshots):
    if s != [i, i * 10, i * 100]:
        failures += 1
        if failures <= 3:
            print("FAIL snapshot", i, "got", s)

# (2) Nested calls of differing arity stress multiple outstanding borrowed
#     buffers at once (f calls g calls h).
def add(*a):
    t = 0
    for x in a:
        t += x
    return t

def outer(a, b, c, d, e, f):
    return add(a, b, c) + add(d, e) + add(f)

if outer(1, 2, 3, 4, 5, 6) != (6 + 9 + 6):
    failures += 1

# (3) Unhappy path: an error while evaluating a call's argument must release the
#     borrowed buffer (not leak/poison the pool). A subsequent call must work.
def inc(x):
    return x + 1

saw_error = False
try:
    inc(undefined_variable)
except Exception:
    saw_error = True
if not saw_error:
    failures += 1
if inc(41) != 42:  # pool must still serve correct buffers after the error
    failures += 1

# (4) Builtin calls borrow/release a buffer too; exercise after the error path.
if len([1, 2, 3, 4]) != 4:
    failures += 1
if sorted([3, 1, 2]) != [1, 2, 3]:
    failures += 1

# (5) Mixed sizes in a tight loop: tiny and larger calls interleaved, so the
#     free-list hands out buffers of varying capacity.
def two(a, b):
    return a + b
def six(a, b, c, d, e, f):
    return a + b + c + d + e + f

total = 0
for i in range(1000):
    total += two(i, i) + six(1, 2, 3, 4, 5, 6)
# total = sum(2*i for i in range(1000)) + 1000*21 = 999000 + 21000 = 1020000
if total != 1020000:
    failures += 1
    print("FAIL mixed-size loop got", total)

assert failures == 0
