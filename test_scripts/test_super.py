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
print("Dog name:", d.name)
print("Dog breed:", d.breed)
print("Dog speak:", d.speak())

g = GoldenRetriever("Goldie")
print("Golden name:", g.name)
print("Golden breed:", g.breed)
print("Golden speak:", g.speak())

True
