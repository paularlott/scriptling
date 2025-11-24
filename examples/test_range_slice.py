# Test range() function
print("=== Testing range() ===")
print("range(5):")
for i in range(5):
    print(i)

print("\nrange(2, 7):")
for i in range(2, 7):
    print(i)

print("\nrange(0, 10, 2):")
for i in range(0, 10, 2):
    print(i)

# Test slice notation
print("\n=== Testing slice notation ===")
numbers = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
print("Original list:", numbers)
print("numbers[2:5]:", numbers[2:5])
print("numbers[:3]:", numbers[:3])
print("numbers[7:]:", numbers[7:])

text = "Hello World"
print("\nOriginal string:", text)
print("text[0:5]:", text[0:5])
print("text[6:]:", text[6:])
print("text[:5]:", text[:5])

# Test dictionary methods
print("\n=== Testing dictionary methods ===")
person = {"name": "Alice", "age": "30", "city": "NYC"}
print("Dictionary:", person)
print("keys():", keys(person))
print("values():", values(person))
print("items():", items(person))

# Iterate over dict items
print("\nIterating over items:")
for item in items(person):
    print("  Key:", item[0], "Value:", item[1])

print("\n=== All tests passed! ===")