# Example: list append method

print("List Append")

# Basic append
numbers = [1, 2, 3]
print(f"Initial: {numbers}")
numbers.append(4)
print(f"After append(4): {numbers}")

# Multiple appends
numbers.append(5)
numbers.append(6)
print(f"After more appends: {numbers}")

# Append different types
mixed = []
mixed.append(1)
mixed.append("two")
mixed.append(3.0)
print(f"Mixed list: {mixed}")

# Append in loop
result = []
for i in range(5):
    result.append(i * 2)
print(f"Built with append: {result}")

# Append list to list
nested = [1, 2]
nested.append([3, 4])
print(f"Nested: {nested}")

