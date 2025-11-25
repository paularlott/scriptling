# Test: range and list slicing

print("=== Testing Range and Slice ===")

# Range basic
print("range(5):")
for i in range(5):
    print(f"  {i}")

# Range with start and stop
print("range(2, 7):")
for i in range(2, 7):
    print(f"  {i}")

# List slicing
numbers = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]
print(f"Full list: {numbers}")

slice1 = numbers[2:5]
print(f"Slice [2:5]: {slice1}")

slice2 = numbers[0:3]
print(f"Slice [0:3]: {slice2}")

slice3 = numbers[5:10]
print(f"Slice [5:10]: {slice3}")

# Slice with strings
text = "Hello, World!"
part = text[0:5]
print(f"String slice [0:5]: {part}")

part2 = text[7:12]
print(f"String slice [7:12]: {part2}")

print("✓ All range/slice tests passed")
