# Tests for dunder methods on user classes (§1.3)

# ── __str__ / __repr__ ────────────────────────────────────────────────────────

class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __str__(self):
        return f"Point({self.x}, {self.y})"

    def __repr__(self):
        return f"Point(x={self.x}, y={self.y})"

p = Point(3, 4)
assert str(p) == "Point(3, 4)", f"str(p) = {str(p)}"
assert repr(p) == "Point(x=3, y=4)", f"repr(p) = {repr(p)}"

# f-string should use __str__
s = f"{p}"
assert s == "Point(3, 4)", f"f-string = {s}"

# print should use __str__ (capture via str())
assert str(p) == "Point(3, 4)"

# ── __str__ fallback to __repr__ ──────────────────────────────────────────────

class OnlyRepr:
    def __repr__(self):
        return "OnlyRepr()"

r = OnlyRepr()
assert repr(r) == "OnlyRepr()", f"repr = {repr(r)}"

# ── __len__ ───────────────────────────────────────────────────────────────────

class Stack:
    def __init__(self):
        self.items = []

    def push(self, item):
        self.items.append(item)

    def __len__(self):
        return len(self.items)

s = Stack()
assert len(s) == 0, f"len(empty stack) = {len(s)}"
s.push(1)
s.push(2)
s.push(3)
assert len(s) == 3, f"len(stack) = {len(s)}"

# ── __bool__ ──────────────────────────────────────────────────────────────────

class Flag:
    def __init__(self, value):
        self.value = value

    def __bool__(self):
        return self.value

f_true = Flag(True)
f_false = Flag(False)
assert bool(f_true) == True
assert bool(f_false) == False
assert f_true  # truthy
assert not f_false  # falsy

# if/while should use __bool__
result = "yes" if f_true else "no"
assert result == "yes"
result = "yes" if f_false else "no"
assert result == "no"

# ── __bool__ via __len__ ──────────────────────────────────────────────────────

s2 = Stack()
assert not s2  # empty stack is falsy via __len__
s2.push(42)
assert s2  # non-empty stack is truthy via __len__

# ── __eq__ / __lt__ ───────────────────────────────────────────────────────────

class Version:
    def __init__(self, major, minor):
        self.major = major
        self.minor = minor

    def __eq__(self, other):
        return self.major == other.major and self.minor == other.minor

    def __lt__(self, other):
        if self.major != other.major:
            return self.major < other.major
        return self.minor < other.minor

v1 = Version(1, 0)
v2 = Version(1, 0)
v3 = Version(2, 0)
v4 = Version(1, 5)

assert v1 == v2, "v1 == v2"
assert not (v1 == v3), "v1 != v3"
assert v1 < v3, "v1 < v3"
assert not (v3 < v1), "not v3 < v1"
assert v1 < v4, "v1 < v4"

# sorted() uses __lt__
versions = [v3, v1, v4]
sorted_v = sorted(versions)
assert sorted_v[0] == v1, "sorted[0] == v1"
assert sorted_v[1] == v4, "sorted[1] == v4"
assert sorted_v[2] == v3, "sorted[2] == v3"

# ── __contains__ ─────────────────────────────────────────────────────────────

class NumberSet:
    def __init__(self, *nums):
        self.nums = list(nums)

    def __contains__(self, item):
        return item in self.nums

ns = NumberSet(1, 2, 3, 5, 8)
assert 1 in ns
assert 3 in ns
assert 4 not in ns
assert 8 in ns

# ── __iter__ / __next__ ───────────────────────────────────────────────────────

class CountUp:
    def __init__(self, start, stop):
        self.start = start
        self.stop = stop

    def __iter__(self):
        return CountUpIterator(self.start, self.stop)

class CountUpIterator:
    def __init__(self, current, stop):
        self.current = current
        self.stop = stop

    def __next__(self):
        if self.current >= self.stop:
            raise StopIteration()
        val = self.current
        self.current = self.current + 1
        return val

counter = CountUp(1, 5)
collected = []
for n in counter:
    collected.append(n)
assert collected == [1, 2, 3, 4], f"collected = {collected}"

# list comprehension with __iter__
doubled = [x * 2 for x in CountUp(0, 4)]
assert doubled == [0, 2, 4, 6], f"doubled = {doubled}"

# ── __iter__ returning self ───────────────────────────────────────────────────

class Range:
    def __init__(self, n):
        self.n = n
        self.i = 0

    def __iter__(self):
        self.i = 0
        return self

    def __next__(self):
        if self.i >= self.n:
            raise StopIteration()
        val = self.i
        self.i = self.i + 1
        return val

r = Range(3)
result = []
for x in r:
    result.append(x)
assert result == [0, 1, 2], f"range result = {result}"

# Can iterate again (reset via __iter__)
result2 = []
for x in r:
    result2.append(x)
assert result2 == [0, 1, 2], f"range result2 = {result2}"

# ── Inheritance of dunder methods ─────────────────────────────────────────────

class Animal:
    def __init__(self, name):
        self.name = name

    def __str__(self):
        return f"Animal({self.name})"

class Dog(Animal):
    pass  # inherits __str__

d = Dog("Rex")
assert str(d) == "Animal(Rex)", f"inherited __str__ = {str(d)}"

# Override in subclass
class Cat(Animal):
    def __str__(self):
        return f"Cat({self.name})"

c = Cat("Whiskers")
assert str(c) == "Cat(Whiskers)", f"overridden __str__ = {str(c)}"

# ── __init__ inherited via base class chain ───────────────────────────────────

class Base:
    def __init__(self, x):
        self.x = x

class Child(Base):
    pass  # no __init__, inherits Base's

child = Child(42)
assert child.x == 42, f"expected 42, got {child.x}"

# Three-level chain
class GrandChild(Child):
    pass

gc = GrandChild(99)
assert gc.x == 99, f"expected 99, got {gc.x}"

# ── Arithmetic dunder methods ─────────────────────────────────────────────────

class Vec:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __add__(self, other):
        return Vec(self.x + other.x, self.y + other.y)

    def __sub__(self, other):
        return Vec(self.x - other.x, self.y - other.y)

    def __mul__(self, scalar):
        return Vec(self.x * scalar, self.y * scalar)

    def __eq__(self, other):
        return self.x == other.x and self.y == other.y

v1 = Vec(1, 2)
v2 = Vec(3, 4)
assert (v1 + v2) == Vec(4, 6), f"add failed"
assert (v2 - v1) == Vec(2, 2), f"sub failed"
assert (v1 * 3) == Vec(3, 6), f"mul failed"

True
