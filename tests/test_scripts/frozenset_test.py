# Test frozenset(): immutable, hashable-by-content sets

fs = frozenset([2, 1, 1])

# Construction, dedup, repr and str
assert len(fs) == 2
assert repr(fs) == "frozenset({1, 2})"
assert str(fs) == "frozenset({1, 2})"
assert repr(frozenset()) == "frozenset()"
assert sorted(fs) == [1, 2]
assert sorted(frozenset("ab")) == ["a", "b"]
assert not frozenset()
assert fs

# frozenset() from a mutable set: frozen copy, original stays mutable
m = {1, 2}
frozen_copy = frozenset(m)
m.add(3)
assert len(frozen_copy) == 2
assert 3 not in frozen_copy

# Equality: against sets and other frozensets
assert fs == {1, 2}
assert fs == frozenset([1, 2])
assert fs != {1, 3}
assert fs != frozenset([1])

# Membership and iteration
assert 1 in fs and 3 not in fs
assert sorted(x for x in fs) == [1, 2]

# Hashable by content: usable as dict keys and set members
d = {fs: "value"}
assert d[frozenset([1, 2])] == "value"
group = {frozenset([1]), frozenset([1])}
assert len(group) == 1
assert frozenset([1]) in group

# Immutability: mutating methods raise AttributeError
def raises_attribute_error(fn):
    try:
        fn()
        return False
    except AttributeError:
        return True

assert raises_attribute_error(lambda: fs.add(3))
assert raises_attribute_error(lambda: fs.remove(1))
assert raises_attribute_error(lambda: fs.discard(1))
assert raises_attribute_error(lambda: fs.pop())
assert raises_attribute_error(lambda: fs.clear())
assert raises_attribute_error(lambda: fs.update([3]))
assert raises_attribute_error(lambda: fs.intersection_update([1]))

# Non-mutating methods still work and stay frozen
c = fs.copy()
assert c == fs
assert type(c) == "frozenset"
u = fs.union({9})
assert type(u) == "frozenset"
assert u == frozenset([1, 2, 9])
assert fs.issubset({1, 2, 3})
assert fs.issuperset({2})

# Set algebra methods accept any iterable (Python), and the multi-argument
# forms of union/intersection/difference fold left
assert fs.union([9]) == frozenset([1, 2, 9])
assert fs.union([9], {10}) == frozenset([1, 2, 9, 10])
assert fs.intersection([2, 9]) == frozenset([2])
assert fs.intersection([2], {1}) == frozenset()  # ({1,2} ∩ [2]) ∩ {1}
assert fs.difference([2]) == frozenset([1])
assert fs.difference([2], {1}) == frozenset()
assert fs.symmetric_difference([2, 9]) == frozenset([1, 9])
assert fs.issubset([1, 2, 3])
assert fs.issuperset([2])
assert fs.issuperset([9]) is False
assert {1, 2}.union([3]) == {1, 2, 3}
assert {1, 2}.intersection("ab") == set()
assert type(fs.union([9])) == "frozenset"

# Operators: result type follows the left operand, as in Python
big = fs | {3}
assert type(big) == "frozenset"
assert big == frozenset([1, 2, 3])
assert type({3} | fs) != "frozenset"
assert (fs & {2, 9}) == frozenset([2])
assert type(fs & {2}) == "frozenset"
assert (fs - {2}) == frozenset([1])
assert (fs ^ {2, 9}) == frozenset([1, 9])

# Unhashable elements are rejected, as for sets
def raises_type_error(fn):
    try:
        fn()
        return False
    except TypeError:
        return True

assert raises_type_error(lambda: frozenset([[1]]))

# set() of a frozenset is a mutable set, as in Python (use .copy() to keep
# it frozen)
mutable = set(fs)
mutable.add(99)
assert 99 in mutable
assert type(mutable) != "frozenset"
assert fs.copy() == fs
assert type(fs.copy()) == "frozenset"

# isinstance distinguishes frozenset from set
assert isinstance(fs, frozenset)
assert not isinstance(fs, set)
assert isinstance({1}, set)
assert not isinstance({1}, frozenset)

# type() and deepcopy interaction covered elsewhere; frozen sets are shared
import copy
assert copy.deepcopy(fs) is fs

True
