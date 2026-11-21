r = range(10)

# slicing gives a range
assert list(r[2:8:2]) == [2, 4, 6]
assert r[2:8:2] == range(2, 8, 2)
assert r[::-1] == range(9, -1, -1)
assert list(r[-3:]) == [7, 8, 9]
assert list(range(5, 0, -1)[1:3]) == [4, 3]
assert str(r[1:4]) == "range(1, 4)"

# indexing, including negative indexes
assert r[0] == 0 and r[-1] == 9 and range(0, 10, 3)[-1] == 9
try:
    r[20]
    assert False, "expected IndexError"
except IndexError:
    pass

# len, in, reversed, ==, index, count, attributes, repr, truth
assert len(range(0, 10, 3)) == 4
assert 3 in r and 10 not in r and 3.0 in r and "a" not in r
assert list(reversed(range(1, 10, 3))) == [7, 4, 1]
assert range(0) == range(2, 2) and range(5) == range(0, 5) and range(3) != [0, 1, 2]
assert r.index(3) == 3 and r.count(3) == 1 and r.count(11) == 0
assert range(-5, 5, 3).count(-2) == 1
try:
    r.index(11)
    assert False, "expected ValueError"
except ValueError:
    pass
rr = range(1, 10, 2)
assert (rr.start, rr.stop, rr.step) == (1, 10, 2)
assert repr(rr) == "range(1, 10, 2)" and str(range(3)) == "range(0, 3)"
assert not range(0) and range(3)

# a range is re-iterable, unlike an iterator
r4 = range(4)
assert list(r4) == [0, 1, 2, 3] and list(r4) == [0, 1, 2, 3]
total = 0
for i in r4:
    for j in r4:
        total += 1
assert total == 16
for i in r4:
    if i == 1:
        break
assert list(r4) == [0, 1, 2, 3] and len(r4) == 4 and r4[1] == 1
assert list(enumerate(r4))[3] == (3, 3)
assert sorted(range(3), reverse=True) == [2, 1, 0] and max(range(5)) == 4
assert type(iter(range(3))).__name__ == "range_iterator"
it = iter(r4)
assert next(it) == 0 and list(it) == [1, 2, 3] and list(r4) == [0, 1, 2, 3]
try:
    next(range(3))
    assert False, "expected TypeError"
except TypeError:
    pass

print("OK")
