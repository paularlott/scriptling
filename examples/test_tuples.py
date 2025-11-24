# Tuple Literals Test

# Empty tuple
empty = ()
print("Empty tuple:", empty)

# Single element tuple (needs trailing comma)
single = (42,)
print("Single tuple:", single)

# Multiple element tuple
point = (1, 2)
print("Point:", point)

# Tuple with mixed types
mixed = ("Alice", 30, True)
print("Mixed tuple:", mixed)

# Tuple indexing
print("Point x:", point[0])
print("Point y:", point[1])
print("Name:", mixed[0])
print("Age:", mixed[1])

# Tuple iteration
print("Iterating over point:")
for coord in point:
    print("Coordinate:", coord)

print("Iterating over mixed:")
for item in mixed:
    print("Item:", item)

# Nested tuples
nested = ((1, 2), (3, 4))
print("Nested:", nested)
print("First pair:", nested[0])
print("Second pair:", nested[1])

# Tuple unpacking (using existing multiple assignment)
x, y = point
print("Unpacked x:", x)
print("Unpacked y:", y)

name, age, active = mixed
print("Unpacked name:", name)
print("Unpacked age:", age)
print("Unpacked active:", active)

print("Tuples test completed")