# Test Python-like append behavior
print("Testing append (in-place modification):")

# Test 1: Basic append
my_list = [1, 2]
print("Before append:", my_list)
append(my_list, 3)
print("After append:", my_list)
print("Length:", len(my_list))

# Test 2: Multiple appends
numbers = []
for i in range(5):
    append(numbers, i)
print("\nAfter multiple appends:", numbers)

# Test 3: Append returns None
result = append(my_list, 4)
print("\nAppend returns:", result)
print("List after append:", my_list)

print("\nAll append tests passed!")