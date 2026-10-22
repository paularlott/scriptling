# collections.deque behaves as Python 3's: this script also runs unchanged
# under CPython 3.14.
import collections, copy

D = collections.deque

d = D([1, 2, 3])
assert isinstance(d, collections.deque)
assert type(d).__name__ == "deque"
assert repr(d) == "deque([1, 2, 3])"
assert str(D(["a", 1], 3)) == "deque(['a', 1], maxlen=3)"
assert d.maxlen is None and D([], 3).maxlen == 3

# Both ends.
d.appendleft(0)
d.append(4)
assert list(d) == [0, 1, 2, 3, 4]
assert d.popleft() == 0 and d.pop() == 4
d.extendleft([9, 8])
assert list(d) == [8, 9, 1, 2, 3]
d.rotate(2)
assert list(d) == [2, 3, 8, 9, 1]
d.rotate(-2)
assert list(d) == [8, 9, 1, 2, 3]

# Sequence methods.
d.insert(1, 7)
assert list(d) == [8, 7, 9, 1, 2, 3]
d.insert(-1, 6)
assert list(d) == [8, 7, 9, 1, 2, 6, 3]
d.insert(99, 5)
assert d[-1] == 5
d.remove(9)
assert 9 not in d and 7 in d
assert d.index(2) == 3
assert D([1, 2, 1, 2]).index(2, 2) == 3
assert D([1, 2, 1, 2]).index(2, -2, 4) == 3
d.reverse()
assert list(d) == [5, 3, 6, 2, 1, 7, 8]
d[0] = 50
del d[1]
assert list(d) == [50, 6, 2, 1, 7, 8]
assert list(reversed(d)) == [8, 7, 1, 2, 6, 50]
assert D([[1], [1], 2]).count([1]) == 2
assert [1] in D([[1]])

# Comparison.
assert D([1, 2]) == D([1, 2])
assert D([1], 5) == D([1])
assert D([1, 2]) != [1, 2]
assert D([1, 2]) < D([1, 3]) and D([2]) > D([1, 9]) and D([1]) <= D([1])
x = D([1])
x.append(x)
assert x == x

# Arithmetic.
assert list(D([1, 2]) + D([3])) == [1, 2, 3]
assert list(D([1, 2]) * 2) == [1, 2, 1, 2]
assert list(2 * D([1])) == [1, 1]
assert D([1, 2, 3], 4) * 2 == D([3, 1, 2, 3])
e = D([1])
f = e
e += [2, 3]
e *= 2
assert e is f and list(e) == [1, 2, 3, 1, 2, 3]

# maxlen.
b = D(range(10), 3)
assert list(b) == [7, 8, 9]
b.appendleft(0)
assert list(b) == [0, 7, 8]
assert D([], 0) == D()

# Copies.
c = D([[1]], 2)
s = copy.copy(c)
dc = copy.deepcopy(c)
c[0].append(2)
c.append(3)
assert list(s) == [[1, 2]] and list(dc) == [[1]] and dc.maxlen == 2

class Sub(collections.deque):
    pass
sd = Sub([1, 2])
assert isinstance(sd, collections.deque) and repr(sd) == "Sub([1, 2])"
assert type(sd.copy()).__name__ == "Sub"

# Errors, as in Python.
def raises(exc, f):
    try:
        f()
    except exc:
        return True
    return False

assert raises(IndexError, lambda: D().pop())
assert raises(IndexError, lambda: D().popleft())
assert raises(IndexError, lambda: D([1])[5])
assert raises(IndexError, lambda: D([1], 1).insert(0, 2))
assert raises(ValueError, lambda: D([1]).remove(5))
assert raises(ValueError, lambda: D([1]).index(5))
assert raises(ValueError, lambda: D([], -1))
assert raises(TypeError, lambda: D(None))
assert raises(TypeError, lambda: D([1]) + [2])
assert raises(TypeError, lambda: D([1]) < [1])
assert raises(TypeError, lambda: hash(D()))
assert raises(AttributeError, lambda: setattr(D(), "maxlen", 3))
assert raises(TypeError, lambda: D([1])[0:1])

def mutate_while_iterating():
    q = D([1, 2])
    for v in q:
        q.append(v)
assert raises(RuntimeError, mutate_while_iterating)
