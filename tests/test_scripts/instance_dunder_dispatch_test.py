import math

# Operators and builtins reach a user class's dunder methods.
class V:
    def __init__(self, v):
        self.v = v
    def __pow__(self, o):
        return ("pow", self.v, o)
    def __rpow__(self, o):
        return ("rpow", self.v, o)
    def __xor__(self, o):
        return ("xor", self.v, o)
    def __rxor__(self, o):
        return ("rxor", self.v, o)
    def __lshift__(self, o):
        return ("lshift", self.v, o)
    def __rlshift__(self, o):
        return ("rlshift", self.v, o)
    def __rshift__(self, o):
        return ("rshift", self.v, o)
    def __rrshift__(self, o):
        return ("rrshift", self.v, o)
    def __lt__(self, o):
        return self.v < (o.v if isinstance(o, V) else o)
    def __gt__(self, o):
        return self.v > (o.v if isinstance(o, V) else o)
    def __le__(self, o):
        return self.v <= (o.v if isinstance(o, V) else o)
    def __ge__(self, o):
        return self.v >= (o.v if isinstance(o, V) else o)
    def __round__(self, n=None):
        return ("round", n)
    def __divmod__(self, o):
        return ("divmod", self.v, o)
    def __rdivmod__(self, o):
        return ("rdivmod", self.v, o)
    def __floor__(self):
        return "floor"
    def __ceil__(self):
        return "ceil"
    def __trunc__(self):
        return "trunc"
    def __int__(self):
        return 7
    def __float__(self):
        return 7.5

assert V(2) ** 3 == ("pow", 2, 3)
assert 2 ** V(3) == ("rpow", 3, 2)
assert V(1) ^ 1 == ("xor", 1, 1)
assert 1 ^ V(1) == ("rxor", 1, 1)
assert V(1) << 2 == ("lshift", 1, 2)
assert 2 << V(1) == ("rlshift", 1, 2)
assert V(8) >> 1 == ("rshift", 8, 1)
assert 1 >> V(8) == ("rrshift", 8, 1)

# A builtin on the left of a comparison uses the instance's reflected method.
assert 1 < V(2) and not (3 < V(2))
assert 3 > V(2) and 1.5 <= V(2) and 2 >= V(2)
# ... and so does sorting a mix of numbers and instances.
assert [x if isinstance(x, int) else x.v for x in sorted([V(3), 1, V(2), 4])] == [1, 2, 3, 4]
assert max(1, V(5)).v == 5 and min(V(0), 1).v == 0

assert round(V(1)) == ("round", None)
assert round(V(1), 2) == ("round", 2)
assert divmod(V(7), 2) == ("divmod", 7, 2)
assert divmod(7, V(2)) == ("rdivmod", 2, 7)
assert math.floor(V(1)) == "floor" and math.ceil(V(1)) == "ceil" and math.trunc(V(1)) == "trunc"
assert "%d" % V(1) == "7"
assert "%.1f" % V(1) == "7.5"

class Idx:
    def __index__(self):
        return 255
assert "%x" % Idx() == "ff"

class OnlyFloat:
    def __float__(self):
        return 2.5
assert math.floor(OnlyFloat()) == 2 and math.ceil(OnlyFloat()) == 3

print("OK")
