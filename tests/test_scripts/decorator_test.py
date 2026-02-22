# Decorator tests

# --- Basic function decorator ---
def double_result(fn):
    def wrapper(*args):
        return fn(*args) * 2
    return wrapper

@double_result
def add(a, b):
    return a + b

assert add(3, 4) == 14, f"expected 14, got {add(3, 4)}"

# --- Stacked decorators (applied inner-first) ---
def add_one(fn):
    def wrapper(*args):
        return fn(*args) + 1
    return wrapper

@add_one
@double_result
def mul(a, b):
    return a * b

# mul(2,3) -> double_result wraps first -> result*2 -> add_one wraps outer -> result+1
# mul(2,3) = 6, *2 = 12, +1 = 13
assert mul(2, 3) == 13, f"expected 13, got {mul(2, 3)}"

# --- @property ---
class Circle:
    def __init__(self, radius):
        self._radius = radius

    @property
    def radius(self):
        return self._radius

    @property
    def area(self):
        return 3.14159 * self._radius * self._radius

c = Circle(5)
assert c.radius == 5, f"expected 5, got {c.radius}"
area = c.area
assert area > 78.5 and area < 78.6, f"unexpected area {area}"

# --- @property inheritance ---
class Shape:
    def __init__(self, name):
        self._name = name

    @property
    def name(self):
        return self._name

class Square(Shape):
    def __init__(self, side):
        super().__init__("square")
        self._side = side

    @property
    def perimeter(self):
        return self._side * 4

sq = Square(3)
assert sq.name == "square", f"expected 'square', got {sq.name}"
assert sq.perimeter == 12, f"expected 12, got {sq.perimeter}"

# --- @staticmethod ---
class MathHelper:
    @staticmethod
    def square(x):
        return x * x

    @staticmethod
    def cube(x):
        return x * x * x

assert MathHelper.square(4) == 16, f"expected 16"
assert MathHelper.cube(3) == 27, f"expected 27"

# Also callable on instance
m = MathHelper()
assert m.square(5) == 25, f"expected 25"

# --- Decorator that preserves identity (identity decorator) ---
def identity(fn):
    return fn

@identity
def greet(name):
    return "hello " + name

assert greet("world") == "hello world"

# --- Class decorator ---
def add_greeting(cls):
    cls.greeting = "hi"
    return cls

@add_greeting
class Greeter:
    pass

g = Greeter()
assert g.greeting == "hi", f"expected 'hi', got {g.greeting}"

# --- @property with setter ---
class Temperature:
    def __init__(self, celsius):
        self._celsius = celsius

    @property
    def celsius(self):
        return self._celsius

    @celsius.setter
    def celsius(self, value):
        if value < -273.15:
            raise ValueError("Temperature below absolute zero")
        self._celsius = value

    @property
    def fahrenheit(self):
        return self._celsius * 9 / 5 + 32

t = Temperature(100)
assert t.celsius == 100, f"expected 100, got {t.celsius}"
assert t.fahrenheit == 212, f"expected 212, got {t.fahrenheit}"
t.celsius = 0
assert t.celsius == 0, f"expected 0 after set, got {t.celsius}"
assert t.fahrenheit == 32, f"expected 32, got {t.fahrenheit}"

# read-only property raises error on assignment
try:
    t.fahrenheit = 100
    assert False, "should have raised"
except Exception as e:
    assert "read-only" in str(e), f"unexpected error: {e}"

# --- @property setter inheritance ---
class Base:
    def __init__(self, v):
        self._v = v

    @property
    def value(self):
        return self._v

    @value.setter
    def value(self, v):
        self._v = v

class Child(Base):
    pass

c = Child(10)
assert c.value == 10
c.value = 20
assert c.value == 20, f"expected 20, got {c.value}"

print("All decorator tests passed")
