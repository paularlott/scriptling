# Integration patterns — cross-feature interactions
# Each section combines multiple language features to expose subtle evaluator bugs.

# ── 1. Custom iterable in comprehension with filter and method call ────────────

class Range:
    def __init__(self, start, stop):
        self.start = start
        self.stop = stop
        self.i = start

    def __iter__(self):
        self.i = self.start
        return self

    def __next__(self):
        if self.i >= self.stop:
            raise StopIteration()
        v = self.i
        self.i = self.i + 1
        return v

    def __len__(self):
        return max(0, self.stop - self.start)

r = Range(0, 10)
evens = [x for x in r if x % 2 == 0]
assert evens == [0, 2, 4, 6, 8]

# reusable — __iter__ resets
odds = [x for x in r if x % 2 != 0]
assert odds == [1, 3, 5, 7, 9]

# len() via __len__
assert len(r) == 10

# nested comprehension over two custom iterables
pairs = [(x, y) for x in Range(0, 3) for y in Range(0, 3) if x != y]
assert len(pairs) == 6
assert (0, 1) in pairs
assert (2, 0) in pairs

# ── 2. Decorator on a class method ───────────────────────────────────────────

def logged(fn):
    def wrapper(self, *args):
        self.log.append(fn.__name__)
        return fn(self, *args)
    return wrapper

class Service:
    def __init__(self):
        self.log = []

    @logged
    def start(self):
        return "started"

    @logged
    def stop(self):
        return "stopped"

s = Service()
assert s.start() == "started"
assert s.stop() == "stopped"
assert s.start() == "started"
assert s.log == ["start", "stop", "start"]

# ── 3. Closure inside a class method ─────────────────────────────────────────

class Accumulator:
    def __init__(self):
        self.total = 0

    def make_adder(self, n):
        def add():
            self.total = self.total + n
        return add

a = Accumulator()
add5 = a.make_adder(5)
add3 = a.make_adder(3)
add5()
add5()
add3()
assert a.total == 13

# ── 4. Exception handling inside a comprehension ─────────────────────────────

def safe_int(s):
    try:
        return int(s)
    except:
        return None

results = [safe_int(x) for x in ["1", "two", "3", "four", "5"]]
assert results == [1, None, 3, None, 5]

# filter out failures
valid = [x for x in results if x is not None]
assert valid == [1, 3, 5]

# ── 5. sorted() with key calling a method on a custom class ──────────────────

class Student:
    def __init__(self, name, grade):
        self.name = name
        self.grade = grade

    def score(self):
        return self.grade

    def __eq__(self, other):
        return self.name == other.name and self.grade == other.grade

students = [
    Student("Charlie", 85),
    Student("Alice", 92),
    Student("Bob", 78),
]

by_score = sorted(students, key=lambda s: s.score())
assert by_score[0].name == "Bob"
assert by_score[1].name == "Charlie"
assert by_score[2].name == "Alice"

by_name = sorted(students, key=lambda s: s.name)
assert by_name[0].name == "Alice"
assert by_name[1].name == "Bob"
assert by_name[2].name == "Charlie"

by_score_desc = sorted(students, key=lambda s: s.score(), reverse=True)
assert by_score_desc[0].name == "Alice"

# ── 6. match inside a loop with break/continue ───────────────────────────────

events = ["start", "data", "error", "data", "end", "data"]
processed = []
errors = 0

for event in events:
    match event:
        case "start":
            processed.append("INIT")
        case "end":
            processed.append("DONE")
            break
        case "error":
            errors += 1
            continue
        case "data":
            processed.append("DATA")

assert processed == ["INIT", "DATA", "DATA", "DONE"]
assert errors == 1

# ── 7. Three-level inheritance with super() at each level ────────────────────

class Vehicle:
    def __init__(self, make):
        self.make = make
        self.features = ["engine"]

    def describe(self):
        return f"{self.make}"

class Car(Vehicle):
    def __init__(self, make, model):
        super().__init__(make)
        self.model = model
        self.features.append("wheels")

    def describe(self):
        return super().describe() + f" {self.model}"

class ElectricCar(Car):
    def __init__(self, make, model, range_km):
        super().__init__(make, model)
        self.range_km = range_km
        self.features.append("battery")

    def describe(self):
        return super().describe() + f" (electric, {self.range_km}km)"

ec = ElectricCar("Tesla", "Model 3", 500)
assert ec.make == "Tesla"
assert ec.model == "Model 3"
assert ec.range_km == 500
assert ec.describe() == "Tesla Model 3 (electric, 500km)"
assert ec.features == ["engine", "wheels", "battery"]

# isinstance across the chain
assert isinstance(ec, "ElectricCar")
assert isinstance(ec, "Car")
assert isinstance(ec, "Vehicle")
assert issubclass(ElectricCar, Car)
assert issubclass(ElectricCar, Vehicle)
assert issubclass(Car, Vehicle)
assert not issubclass(Vehicle, Car)

# ── 8. Class variable vs instance variable mutation ───────────────────────────

class Counter:
    count = 0  # class variable

    def __init__(self, name):
        self.name = name
        Counter.count = Counter.count + 1

    @classmethod
    def reset(cls):
        cls.count = 0

    @classmethod
    def get_count(cls):
        return cls.count

Counter.reset()
a = Counter("a")
b = Counter("b")
c = Counter("c")
assert Counter.get_count() == 3
assert a.count == 3   # reads class variable
assert b.count == 3

# instance variable shadows class variable
a.count = 99
assert a.count == 99       # instance variable
assert Counter.count == 3  # class variable unchanged
assert b.count == 3        # other instances unaffected

Counter.reset()
assert Counter.count == 0
assert b.count == 0    # b still reads class variable
assert a.count == 99   # a has its own instance variable

# ── 9. Memoisation decorator applied to recursive function ────────────────────

def memoize(fn):
    cache = {}
    def wrapper(n):
        if n not in cache:
            cache[n] = fn(n)
        return cache[n]
    return wrapper

call_count = 0

@memoize
def fib(n):
    global call_count
    call_count += 1
    if n <= 1:
        return n
    return fib(n - 1) + fib(n - 2)

call_count = 0
result = fib(10)
assert result == 55
# With memoisation each value computed exactly once
assert call_count == 11  # fib(0)..fib(10)

# Cached — no new calls
fib(10)
fib(9)
assert call_count == 11

# ── 10. Generator expression over a dict view with transformation ─────────────

inventory = {"apple": 5, "banana": 0, "cherry": 12, "date": 0, "elderberry": 3}

in_stock = [k for k, v in inventory.items() if v > 0]
assert sorted(in_stock) == ["apple", "cherry", "elderberry"]

total = sum(v for v in inventory.values())
assert total == 20

restocked = {k: v + 10 for k, v in inventory.items() if v == 0}
assert restocked == {"banana": 10, "date": 10}

# ── 11. Nonlocal in a method returned from a class ───────────────────────────

class StateMachine:
    def __init__(self):
        self.state = "idle"

    def make_transition(self, from_state, to_state):
        def transition():
            if self.state == from_state:
                self.state = to_state
                return True
            return False
        return transition

sm = StateMachine()
start = sm.make_transition("idle", "running")
stop = sm.make_transition("running", "idle")
pause = sm.make_transition("running", "paused")

assert start() == True
assert sm.state == "running"
assert start() == False   # already running
assert pause() == True
assert sm.state == "paused"
assert stop() == False    # not running
sm.state = "running"
assert stop() == True
assert sm.state == "idle"

# ── 12. __contains__ + __iter__ + __len__ working together ───────────────────

class Bag:
    def __init__(self, *items):
        self._items = list(items)

    def __len__(self):
        return len(self._items)

    def __contains__(self, item):
        return item in self._items

    def __iter__(self):
        return iter(self._items)

    def add(self, item):
        self._items.append(item)

bag = Bag(1, 2, 3)
assert len(bag) == 3
assert 2 in bag
assert 5 not in bag

bag.add(5)
assert 5 in bag
assert len(bag) == 4

total = sum(x for x in bag)
assert total == 11

doubled = [x * 2 for x in bag]
assert doubled == [2, 4, 6, 10]

# ── 13. Exception raised and caught across function call boundaries ───────────

def level3(x):
    if x < 0:
        raise ValueError(f"negative: {x}")
    return x * 2

def level2(x):
    return level3(x - 1)

def level1(x):
    try:
        return level2(x)
    except ValueError as e:
        return f"caught: {str(e)}"

assert level1(5) == 8
assert level1(1) == 0
assert level1(0) == "caught: negative: -1"
assert level1(-1) == "caught: negative: -2"

# ── 14. Lambda as default argument and in data structures ────────────────────

ops = {
    "add": lambda a, b: a + b,
    "sub": lambda a, b: a - b,
    "mul": lambda a, b: a * b,
}

assert ops["add"](3, 4) == 7
assert ops["sub"](10, 3) == 7
assert ops["mul"](3, 4) == 12

# pipeline of lambdas
pipeline = [
    lambda x: x * 2,
    lambda x: x + 1,
    lambda x: x ** 2,
]

result = 3
for fn in pipeline:
    result = fn(result)
# (3*2=6, 6+1=7, 7**2=49)
assert result == 49

# ── 15. Property + inheritance + exception ────────────────────────────────────

class PositiveValue:
    def __init__(self, v):
        self._v = v

    @property
    def value(self):
        return self._v

    @value.setter
    def value(self, v):
        if v < 0:
            raise ValueError("must be positive")
        self._v = v

class ScaledValue(PositiveValue):
    def __init__(self, v, scale):
        super().__init__(v)
        self.scale = scale

    @property
    def scaled(self):
        return self._v * self.scale

sv = ScaledValue(10, 3)
assert sv.value == 10
assert sv.scaled == 30

sv.value = 5
assert sv.value == 5
assert sv.scaled == 15

caught = False
try:
    sv.value = -1
except ValueError:
    caught = True
assert caught
assert sv.value == 5  # unchanged after failed set

# ── 16. Multi-for comprehension with unpacking and condition ─────────────────

matrix = [[1, 2, 3], [4, 5, 6], [7, 8, 9]]

flat = [x for row in matrix for x in row]
assert flat == [1, 2, 3, 4, 5, 6, 7, 8, 9]

diagonal = [matrix[i][i] for i in range(3)]
assert diagonal == [1, 5, 9]

# cross-product with condition
pairs = [(x, y) for x in range(1, 4) for y in range(1, 4) if x != y]
assert len(pairs) == 6
assert (1, 2) in pairs
assert (1, 1) not in pairs

# dict from two lists
keys = ["a", "b", "c"]
vals = [1, 2, 3]
d = {k: v for k, v in zip(keys, vals)}
assert d == {"a": 1, "b": 2, "c": 3}

# ── 17. Context manager + exception suppression + state ──────────────────────

class Transaction:
    def __init__(self):
        self.committed = False
        self.rolled_back = False
        self.ops = []

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        if exc_val is not None:
            self.rolled_back = True
            return True  # suppress exception
        self.committed = True
        return False

    def add(self, op):
        self.ops.append(op)

# successful transaction
t = Transaction()
with t as tx:
    tx.add("insert")
    tx.add("update")
assert t.committed == True
assert t.rolled_back == False
assert t.ops == ["insert", "update"]

# failed transaction — exception suppressed
t2 = Transaction()
with t2 as tx:
    tx.add("insert")
    raise RuntimeError("db error")
    tx.add("update")  # never reached
assert t2.committed == False
assert t2.rolled_back == True
assert t2.ops == ["insert"]

# ── 18. Recursive data structure processing ───────────────────────────────────

def deep_sum(obj):
    if isinstance(obj, int) or isinstance(obj, float):
        return obj
    if isinstance(obj, list):
        return sum(deep_sum(x) for x in obj)
    if isinstance(obj, dict):
        return sum(deep_sum(v) for v in obj.values())
    return 0

assert deep_sum(5) == 5
assert deep_sum([1, 2, 3]) == 6
assert deep_sum([1, [2, [3, 4]], 5]) == 15
assert deep_sum({"a": 1, "b": {"c": 2, "d": 3}}) == 6
assert deep_sum([{"a": 1}, {"b": [2, 3]}]) == 6

def flatten(obj):
    if isinstance(obj, list):
        result = []
        for item in obj:
            result = result + flatten(item)
        return result
    return [obj]

assert flatten([1, [2, [3, [4]]], 5]) == [1, 2, 3, 4, 5]
assert flatten([]) == []

# ── 19. __getitem__ + __setitem__ + __iter__ + __len__ on a custom mapping ────

class TypedDict:
    """Dict that only accepts string keys and integer values."""
    def __init__(self):
        self._data = {}

    def __setitem__(self, key, val):
        if not isinstance(key, str):
            raise TypeError("key must be string")
        if not isinstance(val, int):
            raise TypeError("value must be int")
        self._data[key] = val

    def __getitem__(self, key):
        return self._data[key]

    def __len__(self):
        return len(self._data)

    def __iter__(self):
        return iter(self._data)

    def __contains__(self, key):
        return key in self._data

td = TypedDict()
td["x"] = 1
td["y"] = 2
td["z"] = 3

assert td["x"] == 1
assert len(td) == 3
assert "x" in td
assert "w" not in td

keys = [k for k in td]
assert sorted(keys) == ["x", "y", "z"]

# type enforcement
caught = False
try:
    td[42] = 1
except TypeError:
    caught = True
assert caught

caught = False
try:
    td["a"] = "not an int"
except TypeError:
    caught = True
assert caught

# ── 20. Chained method calls returning self (fluent interface) ────────────────

class QueryBuilder:
    def __init__(self, table):
        self.table = table
        self._filters = []
        self._limit = None
        self._fields = ["*"]

    def select(self, *fields):
        self._fields = list(fields)
        return self

    def where(self, condition):
        self._filters.append(condition)
        return self

    def limit(self, n):
        self._limit = n
        return self

    def build(self):
        q = f"SELECT {', '.join(self._fields)} FROM {self.table}"
        if self._filters:
            q += " WHERE " + " AND ".join(self._filters)
        if self._limit is not None:
            q += f" LIMIT {self._limit}"
        return q

q = (QueryBuilder("users")
     .select("id", "name", "email")
     .where("active = 1")
     .where("age > 18")
     .limit(10)
     .build())

assert q == "SELECT id, name, email FROM users WHERE active = 1 AND age > 18 LIMIT 10"

# minimal query
q2 = QueryBuilder("logs").build()
assert q2 == "SELECT * FROM logs"

# ── 21. Augmented assignment through all types ────────────────────────────────

# int
n = 10
n += 5;  assert n == 15
n -= 3;  assert n == 12
n *= 2;  assert n == 24
n //= 5; assert n == 4
n **= 3; assert n == 64
n %= 10; assert n == 4

# float
f = 1.0
f += 0.5; assert f == 1.5
f *= 2.0; assert f == 3.0
f /= 2.0; assert f == 1.5

# string
s = "hello"
s += " world"
assert s == "hello world"

# list
lst = [1, 2]
lst += [3, 4]
assert lst == [1, 2, 3, 4]

# augmented assignment on instance attribute
class Box:
    def __init__(self, v):
        self.v = v

b = Box(10)
b.v = b.v + 5
assert b.v == 15
b.v = b.v * 2
assert b.v == 30

# augmented assignment on list element
lst = [1, 2, 3]
lst[1] = lst[1] + 10
assert lst == [1, 12, 3]

# augmented assignment on dict value
d = {"x": 5}
d["x"] = d["x"] + 3
assert d["x"] == 8

# ── 22. for/else and while/else with break ────────────────────────────────────

def find_prime(lst):
    def is_prime(n):
        if n < 2:
            return False
        for i in range(2, n):
            if n % i == 0:
                return False
        return True

    for n in lst:
        if is_prime(n):
            return n
    return None

assert find_prime([4, 6, 8, 7]) == 7
assert find_prime([4, 6, 8]) == None

# for/else: else runs only when no break
def has_negative(lst):
    for x in lst:
        if x < 0:
            break
    else:
        return False
    return True

assert has_negative([1, -2, 3]) == True
assert has_negative([1, 2, 3]) == False

# while/else
def countdown_to_zero(start):
    n = start
    while n > 0:
        n -= 1
    else:
        return "reached zero"
    return "broke out"

assert countdown_to_zero(3) == "reached zero"

# ── 23. String methods chained with comprehensions ───────────────────────────

csv = "  Alice, 30, Engineer  \n  Bob, 25, Designer  \n  Charlie, 35, Manager  "

rows = [row.strip() for row in csv.strip().splitlines()]
assert len(rows) == 3

parsed = [
    {"name": p[0].strip(), "age": int(p[1].strip()), "role": p[2].strip()}
    for row in rows
    for p in [row.split(",")]
]

assert parsed[0] == {"name": "Alice", "age": 30, "role": "Engineer"}
assert parsed[1] == {"name": "Bob", "age": 25, "role": "Designer"}
assert parsed[2] == {"name": "Charlie", "age": 35, "role": "Manager"}

names = [p["name"] for p in parsed]
assert names == ["Alice", "Bob", "Charlie"]

seniors = [p["name"] for p in parsed if p["age"] >= 30]
assert seniors == ["Alice", "Charlie"]

# ── 24. Variadic args + kwargs + defaults all together ───────────────────────

def build_tag(tag, cls=None, id=None, **attrs):
    parts = [tag]
    if id:
        parts.append(f'id="{id}"')
    if cls:
        parts.append(f'class="{cls}"')
    for k, v in attrs.items():
        parts.append(f'{k}="{v}"')
    return f"<{' '.join(parts)}></{tag}>"

assert build_tag("p") == "<p></p>"
assert build_tag("p", cls="intro") == '<p class="intro"></p>'
assert build_tag("a", href="https://example.com", id="link1") == '<a id="link1" href="https://example.com"></a>'

# ── 25. Hash-based deduplication using __hash__ + __eq__ ─────────────────────

class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __hash__(self):
        return self.x * 31 + self.y

    def __eq__(self, other):
        return self.x == other.x and self.y == other.y

    def __repr__(self):
        return f"Point({self.x},{self.y})"

points = [Point(1,2), Point(3,4), Point(1,2), Point(5,6), Point(3,4)]

# dedup via set
unique = list({p: True for p in points}.keys())
assert len(unique) == 3

# use as dict keys
distances = {}
for p in points:
    distances[p] = (p.x**2 + p.y**2) ** 0.5

assert len(distances) == 3
assert distances[Point(1,2)] == distances[Point(1,2)]

# sorted by distance
sorted_pts = sorted(unique, key=lambda p: p.x**2 + p.y**2)
assert sorted_pts[0] == Point(1,2)
assert sorted_pts[2] == Point(5,6)

print("All integration pattern tests passed!")
