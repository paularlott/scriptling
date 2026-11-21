from abc import ABC, abstractmethod
import abc


class Shape(ABC):
    @abstractmethod
    def area(self):
        pass

    @abstractmethod
    def perimeter(self):
        pass

    def describe(self):
        return "area %s" % self.area()


class Half(Shape):
    def area(self):
        return 1


class Square(Half):
    def perimeter(self):
        return 4


def raises_type_error(fn):
    try:
        fn()
    except TypeError as e:
        return str(e)
    return None


msg = raises_type_error(lambda: Shape())
assert msg is not None and "Can't instantiate abstract class Shape" in msg, msg
assert "'area'" in msg and "'perimeter'" in msg, msg

msg = raises_type_error(lambda: Half())
assert msg is not None and "Half" in msg and "'perimeter'" in msg and "'area'" not in msg, msg

s = Square()
assert s.describe() == "area 1" and s.perimeter() == 4
assert isinstance(s, Shape) and isinstance(s, ABC)
assert issubclass(Square, Shape) and issubclass(Square, ABC)


# an ABC with no abstract methods can be instantiated
class NoAbstract(ABC):
    pass


assert type(NoAbstract()).__name__ == "NoAbstract"


# @abstractmethod is not enforced without ABC, as in Python
class Plain:
    @abstractmethod
    def f(self):
        pass


assert type(Plain()).__name__ == "Plain"


# abstract properties
class Named(ABC):
    @property
    @abstractmethod
    def name(self):
        ...


class Impl(Named):
    @property
    def name(self):
        return "impl"


assert raises_type_error(lambda: Named()) is not None
assert Impl().name == "impl"


# __init__ in the abstract base, super() from the subclass
class Task(ABC):
    def __init__(self, v):
        self.v = v

    @abstractmethod
    def run(self):
        ...


class Runner(Task):
    def __init__(self, v):
        super().__init__(v)

    def run(self):
        return self.v * 2


assert Runner(4).run() == 8
assert raises_type_error(lambda: Task(1)) is not None


# abc.ABC via the module
class Base(abc.ABC):
    @abc.abstractmethod
    def go(self):
        ...


assert raises_type_error(lambda: Base()) is not None

print("OK")
