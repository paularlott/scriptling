class Animal:
    def __init__(self, name):
        self.name = name

    def speak(self):
        return "Generic animal sound"

class Dog(Animal):
    def __init__(self, name, breed):
        super(Dog, self).__init__(name)
        self.breed = breed

    def speak(self):
        parent_sound = super(Dog, self).speak()
        return parent_sound + " and Woof!"

class GoldenRetriever(Dog):
    def __init__(self, name):
        super(GoldenRetriever, self).__init__(name, "Golden Retriever")

    def speak(self):
        return "Golden says: " + super(GoldenRetriever, self).speak()

d = Dog("Buddy", "Pug")
assert d.name == "Buddy", "Dog name should be Buddy"
assert d.breed == "Pug", "Dog breed should be Pug"
assert d.speak() == "Generic animal sound and Woof!", "Dog speak should include parent sound and Woof"

g = GoldenRetriever("Goldie")
assert g.name == "Goldie", "Golden name should be Goldie"
assert g.breed == "Golden Retriever", "Golden breed should be Golden Retriever"
assert g.speak() == "Golden says: Generic animal sound and Woof!", "Golden speak should include prefix and parent speak"

print("✓ All super() tests passed")
