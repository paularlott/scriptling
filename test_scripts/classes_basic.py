
class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def move(self, dx, dy):
        self.x = self.x + dx
        self.y = self.y + dy

    def distance_squared(self):
        return self.x * self.x + self.y * self.y

p1 = Point(1, 2)
print(f"p1: ({p1.x}, {p1.y})")
assert p1.x == 1
assert p1.y == 2

p1.move(3, 4)
print(f"p1 moved: ({p1.x}, {p1.y})")
assert p1.x == 4
assert p1.y == 6

dist = p1.distance_squared()
print(f"p1 distance squared: {dist}")
assert dist == 16 + 36

p2 = Point(10, 20)
print(f"p2: ({p2.x}, {p2.y})")
assert p2.x == 10
assert p2.y == 20
assert p1.x == 4 # Ensure p1 is not affected

# Test field assignment outside class
p2.z = 100
print(f"p2.z: {p2.z}")
assert p2.z == 100

print("All class tests passed!")
True
