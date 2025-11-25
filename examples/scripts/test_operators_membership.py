# Test: Membership operators (in, not in)

print("=== Testing Membership Operators ===")

# in operator with lists
numbers = [1, 2, 3, 4, 5]
print(f"3 in {numbers}: {3 in numbers}")
print(f"6 in {numbers}: {6 in numbers}")

# not in operator with lists
print(f"6 not in {numbers}: {6 not in numbers}")
print(f"3 not in {numbers}: {3 not in numbers}")

# in operator with strings
text = "hello world"
print(f"'world' in '{text}': {'world' in text}")
print(f"'python' in '{text}': {'python' in text}")

# in operator with dicts (checks keys)
person = {"name": "Alice", "age": "30"}
print(f"'name' in person: {'name' in person}")
print(f"'email' in person: {'email' in person}")

print("✓ All membership operator tests passed")
