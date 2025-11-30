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
assert c.name == "Alice", "Child name should be Alice"
assert c.age == 10, "Child age should be 10"
assert c.greet() == "Hello from Parent, Alice (Age: 10)", "Child greet should include parent greeting and age"

gc = GrandChild("Bob", 5, "Lego")
assert gc.name == "Bob", "GrandChild name should be Bob"
assert gc.age == 5, "GrandChild age should be 5"
assert gc.toy == "Lego", "GrandChild toy should be Lego"
assert gc.greet() == "Hello from Parent, Bob (Age: 5) playing with Lego", "GrandChild greet should include full chain"

# Test explicit super() still works
c2 = Child("Charlie", 12)
assert super(Child, c2).greet() == "Hello from Parent, Charlie", "Explicit super call should work"

print("✓ All super() no-args tests passed")
