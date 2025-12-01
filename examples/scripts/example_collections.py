# Example: Collections (lists, dicts, tuples)
# Creation, access, modification, methods

print("Collections")

# Lists
numbers = [1, 2, 3, 4, 5]
print(f"List: {numbers}")
print(f"First element: {numbers[0]}")
print(f"Last element: {numbers[4]}")
print(f"Length: {len(numbers)}")

# List modification
numbers.append(6)
print(f"After append(6): {numbers}")

# List slicing
slice_result = numbers[1:4]
print(f"Slice [1:4]: {slice_result}")

# Dictionaries
person = {"name": "Alice", "age": "30", "city": "NYC"}
print(f"Dict: {person}")
print(f"Name: {person['name']}")
print(f"Age: {person['age']}")

# Dict methods
keys = person.keys()
print(f"Keys: {keys}")
values = person.values()
print(f"Values: {values}")

# Dict access
if "name" in person:
    print("Dict contains 'name' key")

# Tuples
coords = (10, 20, 30)
print(f"Tuple: {coords}")
print(f"First coord: {coords[0]}")

# Nested structures
matrix = [[1, 2], [3, 4], [5, 6]]
print(f"Matrix: {matrix}")
print(f"Element [1][1]: {matrix[1][1]}")

# Mixed collection
mixed = [1, "two", 3.0, [4, 5]]
print(f"Mixed list: {mixed}")

