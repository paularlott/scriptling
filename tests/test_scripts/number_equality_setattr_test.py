# Numbers of different types compare equal inside containers, hash alike,
# and __setattr__ is honoured. Behaves the same under CPython 3.

assert {"a": 1} == {"a": 1.0}
assert [1, 2] == [1.0, 2.0]
assert (True, 0) == (1, False)
assert {"k": [1, {"n": 2}]} == {"k": [1.0, {"n": 2.0}]}
assert [1] != [1.5] and {"a": 1} != {"a": 2.0}
assert [1, "1"] != [1, 1]
assert hash(1) == hash(1.0) == hash(True)
assert hash("a") == hash("a") and hash((1, 2)) == hash((1.0, 2))
assert hash(2) != hash(3)
assert {1: "x"}[1.0] == "x" and 1.0 in {1, 2}
assert [1.0, 2].index(1) == 0 and [1, 1.0, True].count(1) == 3
assert 2.0 in [1, 2, 3]
assert (0.5).as_integer_ratio() == (1, 2)
assert isinstance((0.75).as_integer_ratio(), tuple)

class Doubler:
    def __init__(self):
        self.plain = 1
    def __setattr__(self, name, value):
        if isinstance(value, int):
            value = value * 2
        super().__setattr__(name, value)
d = Doubler()
assert d.plain == 2
d.x = 5
assert d.x == 10
d.name = "s"
assert d.name == "s"
setattr(d, "y", 3)
assert d.y == 6
d.x += 1
assert d.x == 22
assert vars(d) == {"plain": 2, "x": 22, "name": "s", "y": 6}

class Locked:
    def __init__(self):
        self.__dict__["ready"] = True
    def __setattr__(self, name, value):
        raise AttributeError("read-only: " + name)
lk = Locked()
try:
    lk.a = 1
    outcome = "set"
except AttributeError as e:
    outcome = str(e)
assert outcome == "read-only: a"
assert lk.ready == True

class Logged:
    log = []
    def __setattr__(self, name, value):
        Logged.log.append(name)
        self.__dict__[name] = value
class Sub(Logged):
    pass
s = Sub()
s.a = 1
s.b = 2
assert Logged.log == ["a", "b"] and s.a + s.b == 3

class Plain:
    pass
p = Plain()
p.v = 4
assert p.v == 4
print("OK")
