# Test class inheritance and tuple unpacking in list comprehensions

# Test basic class inheritance
class Animal:
    def __init__(self, name):
        self.name = name

    def speak(self):
        return "Some sound"

    def describe(self):
        return "I am an animal named " + self.name

class Dog(Animal):
    def __init__(self, name, breed):
        # Manually set parent attributes since super() isn't available
        self.name = name
        self.breed = breed

    def speak(self):
        return "Woof!"

# Test inheritance
dog = Dog("Buddy", "Labrador")
assert dog.name == "Buddy", f"Expected 'Buddy', got {dog.name}"
assert dog.breed == "Labrador", f"Expected 'Labrador', got {dog.breed}"
assert dog.speak() == "Woof!", f"Expected 'Woof!', got {dog.speak()}"
# Inherited method should still work
assert dog.describe() == "I am an animal named Buddy", f"Got {dog.describe()}"

# Test base class directly
animal = Animal("Generic")
assert animal.name == "Generic"
assert animal.speak() == "Some sound"

print("Class inheritance tests passed!")

# Test tuple unpacking in list comprehension
pairs = [("a", 1), ("b", 2), ("c", 3)]
letters = [letter for letter, num in pairs]
assert letters == ["a", "b", "c"], f"Got {letters}"

numbers = [num for letter, num in pairs]
assert numbers == [1, 2, 3], f"Got {numbers}"

# Combined with filter
filtered = [letter for letter, num in pairs if num > 1]
assert filtered == ["b", "c"], f"Got {filtered}"

# More complex tuple unpacking
data = [("Alice", 30, "Engineer"), ("Bob", 25, "Designer")]
names = [name for name, age, job in data]
assert names == ["Alice", "Bob"], f"Got {names}"

ages = [age for name, age, job in data if age > 26]
assert ages == [30], f"Got {ages}"

print("Tuple unpacking tests passed!")
print("All tests passed!")
True
