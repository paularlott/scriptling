# Tests for next(), iter(), dir(), issubclass(), copy() builtins

# --- next() ---

it = iter([10, 20, 30])
assert next(it) == 10
assert next(it) == 20
assert next(it) == 30

# default value on exhaustion
it2 = iter([1])
assert next(it2) == 1
assert next(it2, "done") == "done"

# StopIteration raised when exhausted with no default
it3 = iter([])
caught = False
try:
    next(it3)
except StopIteration:
    caught = True
assert caught

# next() on range iterator
r = iter(range(3))
assert next(r) == 0
assert next(r) == 1

# --- iter() ---

# iter on list returns iterator
it = iter([1, 2, 3])
assert next(it) == 1

# iter on tuple
it = iter((4, 5))
assert next(it) == 4

# iter on string
it = iter("ab")
assert next(it) == "a"
assert next(it) == "b"

# iter on set (just check it works)
s = set([7, 8, 9])
it = iter(s)
vals = []
for v in it:
    vals.append(v)
assert len(vals) == 3

# iter on dict returns key iterator
d = {"x": 1, "y": 2}
keys = list(iter(d))
assert len(keys) == 2

# iter on existing iterator returns same
it = iter([1, 2])
it2 = iter(it)
assert next(it2) == 1

# iter on instance with __iter__
class Counter:
    def __init__(self, n):
        self.n = n
        self.i = 0
    def __iter__(self):
        return self
    def __next__(self):
        if self.i >= self.n:
            raise StopIteration()
        v = self.i
        self.i = self.i + 1
        return v

it = iter(Counter(3))
assert next(it) == 0
assert next(it) == 1
assert next(it) == 2
assert next(it, -1) == -1

# --- dir() ---

# dir() with no args returns list of strings containing known builtins
names = dir()
assert isinstance(names, "list")
assert "len" in names
assert "print" in names
assert "next" in names
assert "iter" in names
assert "dir" in names
assert "copy" in names
assert "issubclass" in names

# dir on instance
class Dog:
    def __init__(self, name):
        self.name = name
    def bark(self):
        return "woof"

d = Dog("Rex")
attrs = dir(d)
assert "name" in attrs
assert "bark" in attrs

# dir on class
attrs = dir(Dog)
assert "bark" in attrs

# dir on dict
d = {"alpha": 1, "beta": 2}
keys = dir(d)
assert "alpha" in keys
assert "beta" in keys

# dir returns sorted list
names = dir()
assert names == sorted(names)

# --- issubclass() ---

class Animal:
    pass

class Dog(Animal):
    pass

class Cat(Animal):
    pass

assert issubclass(Dog, Animal)
assert issubclass(Animal, Animal)  # class is subclass of itself
assert not issubclass(Cat, Dog)
assert not issubclass(Animal, Dog)

# --- copy() ---

# list copy
original = [1, 2, 3]
c = copy(original)
c.append(4)
assert len(original) == 3
assert len(c) == 4

# dict copy
d = {"a": 1, "b": 2}
c = copy(d)
c["c"] = 3
assert len(d) == 2
assert len(c) == 3

# set copy
s = set([1, 2, 3])
c = copy(s)
c.add(4)
assert len(s) == 3
assert len(c) == 4

# tuple copy returns same (immutable)
t = (1, 2, 3)
c = copy(t)
assert len(c) == 3

# instance copy
class Box:
    def __init__(self, v):
        self.v = v

b = Box(10)
c = copy(b)
c.v = 99
assert b.v == 10
assert c.v == 99

# scalar copy returns same value
assert copy(42) == 42
assert copy("hello") == "hello"
