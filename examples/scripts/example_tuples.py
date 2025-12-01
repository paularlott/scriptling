# Example: Tuples

print("Tuples")

# Create tuple
coords = (10, 20, 30)
print(f"Tuple: {coords}")
print(f"Length: {len(coords)}")

# Access elements
print(f"First element: {coords[0]}")
print(f"Second element: {coords[1]}")
print(f"Third element: {coords[2]}")

# Tuple with mixed types
mixed = (1, "two", 3.0, True)
print(f"Mixed tuple: {mixed}")

# Tuple unpacking with multiple assignment
x, y, z = coords
print(f"Unpacked: x={x}, y={y}, z={z}")

# Nested tuples
nested = ((1, 2), (3, 4), (5, 6))
print(f"Nested tuple: {nested}")
print(f"Element [1][0]: {nested[1][0]}")

# Tuple as function return
def get_point():
    return (100, 200)

point = get_point()
print(f"Function returned tuple: {point}")

# Iterate over tuple
print("Iterating over tuple:")
for val in coords:
    print(f"  {val}")

