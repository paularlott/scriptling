import random

# getrandbits(k): a non-negative int below 2**k
assert random.getrandbits(0) == 0
for _ in range(50):
    assert 0 <= random.getrandbits(8) < 256
    assert 0 <= random.getrandbits(1) <= 1
    assert random.getrandbits(63) >= 0
try:
    random.getrandbits(-1)
    assert False
except ValueError:
    pass

# seed() accepts None, ints, floats, str and bytes; the same seed repeats
for s in [42, 3.5, "abc", b"abc"]:
    random.seed(s)
    a = [random.random() for _ in range(3)]
    random.seed(s)
    assert [random.random() for _ in range(3)] == a
random.seed(None)
random.seed()

print("OK")
