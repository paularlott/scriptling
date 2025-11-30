class Parent:
    def __init__(self, name):
        self.name = name

    def greet(self):
        return "Hello from Parent, " + self.name

class Child(Parent):
    def __init__(self, name, age):
        super().__init__(name)
        self.age = age

    def greet(self):
        return super().greet() + " (Age: " + str(self.age) + ")"

class GrandChild(Child):
    def __init__(self, name, age, toy):
        super().__init__(name, age)
        self.toy = toy

    def greet(self):
        return super().greet() + " playing with " + self.toy

# Test parameterless super()
c = Child("Alice", 10)
print("Child name:", c.name)
print("Child age:", c.age)
print("Child greet:", c.greet())

gc = GrandChild("Bob", 5, "Lego")
print("GrandChild name:", gc.name)
print("GrandChild age:", gc.age)
print("GrandChild toy:", gc.toy)
print("GrandChild greet:", gc.greet())

# Test explicit super() still works
c2 = Child("Charlie", 12)
print("Explicit super call:", super(Child, c2).greet())

True
