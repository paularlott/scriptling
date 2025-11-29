
# Test class composition and imports

class Point:
    def __init__(self, x, y):
        self.x = x
        self.y = y

    def __str__(self):
        return "(" + str(self.x) + ", " + str(self.y) + ")"

class Rectangle:
    def __init__(self, p1, p2):
        self.top_left = p1
        self.bottom_right = p2

    def area(self):
        width = self.bottom_right.x - self.top_left.x
        height = self.bottom_right.y - self.top_left.y
        return width * height

    def center(self):
        cx = (self.top_left.x + self.bottom_right.x) / 2
        cy = (self.top_left.y + self.bottom_right.y) / 2
        return Point(cx, cy)

# Test composition
p1 = Point(0, 0)
p2 = Point(10, 20)
rect = Rectangle(p1, p2)

print("Rectangle area:", rect.area())
assert rect.area() == 200

center = rect.center()
print("Rectangle center:", center.x, center.y)
assert center.x == 5
assert center.y == 10

# Test nested property access
try:
    print("Top left x:", rect.top_left.x)
    assert rect.top_left.x == 0
except:
    print("Nested access failed (expected if not implemented)")
    # Workaround
    tl = rect.top_left
    assert tl.x == 0

print("Complex class tests passed!")
True
