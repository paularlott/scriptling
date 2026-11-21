# super() reaching static and class methods on a base class.
# Behaves the same under CPython 3.

class Base:
    registry = "base"
    @staticmethod
    def helper(x, scale=2):
        return x * scale
    @classmethod
    def create(cls, value=1):
        obj = cls()
        obj.value = value
        return obj
    @classmethod
    def label(cls):
        return cls.__name__

class Child(Base):
    def twice(self, x):
        return super().helper(x) + super().helper(x, scale=3)
    def clone(self):
        return super().create(value=7)
    def base_label(self):
        return super().label()
    def explicit(self):
        return super(Child, self).helper(5), super(Child, self).label()

class GrandChild(Child):
    def base_label(self):
        return "g:" + super().base_label()

c = Child()
assert c.twice(2) == 10
made = c.clone()
assert isinstance(made, Child) and made.value == 7
assert c.base_label() == "Child"
assert c.explicit() == (10, "Child")
g = GrandChild()
assert g.base_label() == "g:GrandChild"
assert isinstance(g.clone(), GrandChild)
assert g.twice(1) == 5

def outside():
    return super()
try:
    outside()
    raised = "nothing"
except RuntimeError:
    raised = "RuntimeError"
assert raised == "RuntimeError"
print("OK")
