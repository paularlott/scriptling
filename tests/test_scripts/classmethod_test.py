# Basic @classmethod as factory method
class Date:
    def __init__(self, year, month, day):
        self.year = year
        self.month = month
        self.day = day

    @classmethod
    def from_string(cls, s):
        parts = s.split("-")
        return cls(int(parts[0]), int(parts[1]), int(parts[2]))

    @classmethod
    def today(cls):
        return cls(2024, 1, 1)

    def to_string(self):
        return f"{self.year}-{self.month:02d}-{self.day:02d}"

d = Date.from_string("2024-03-15")
assert d.year == 2024
assert d.month == 3
assert d.day == 15
assert d.to_string() == "2024-03-15"

# Call classmethod on instance
d2 = d.from_string("2023-06-20")
assert d2.year == 2023
assert d2.month == 6
assert d2.day == 20

# today() factory
t = Date.today()
assert t.year == 2024
assert t.month == 1
assert t.day == 1

# @classmethod with inheritance - cls refers to the subclass
class Animal:
    @classmethod
    def create(cls):
        return cls()

    def kind(self):
        return "animal"

class Dog(Animal):
    def kind(self):
        return "dog"

d = Dog.create()
assert d.kind() == "dog"

a = Animal.create()
assert a.kind() == "animal"

# @classmethod called on instance uses instance's class
dog_instance = Dog()
d2 = dog_instance.create()
assert d2.kind() == "dog"

print("All @classmethod tests passed!")
