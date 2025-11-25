# Test: Slice with step parameter
# Tests forward slicing, reverse slicing, and various step values

print("=== Testing Slice with Step ===")

# Test list slicing with step
numbers = [0, 1, 2, 3, 4, 5, 6, 7, 8, 9]

# Every second element
result = numbers[::2]
print("numbers[::2] = " + str(result))

# Every second element starting from index 1
result = numbers[1::2]
print("numbers[1::2] = " + str(result))

# Slice with start, end, and step
result = numbers[1:8:2]
print("numbers[1:8:2] = " + str(result))

# Reverse the list with [::-1]
result = numbers[::-1]
print("numbers[::-1] = " + str(result))

# Every second element in reverse
result = numbers[::-2]
print("numbers[::-2] = " + str(result))

# Reverse slice with start and end
result = numbers[7:2:-1]
print("numbers[7:2:-1] = " + str(result))

# Test string slicing with step
text = "hello world"

# Every second character
result = text[::2]
print("text[::2] = " + result)

# Reverse string
result = text[::-1]
print("text[::-1] = " + result)

# Slice with start, end, and step
result = text[1:9:2]
print("text[1:9:2] = " + result)

# Every second character in reverse
result = text[::-2]
print("text[::-2] = " + result)

print("✓ All slice step tests passed")
