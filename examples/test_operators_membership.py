# Test membership operators (in, not in)
# Priority 1 feature

print("=== Membership Operators Test ===")
print("")

# Test 1: in operator with lists
print("1. in operator with lists")
numbers = [1, 2, 3, 4, 5]
if 3 in numbers:
    print("   ✓ 3 in [1,2,3,4,5]")
if 10 in numbers:
    print("   ✗ Should not find 10")
else:
    print("   ✓ 10 not found in list")
print("")

# Test 2: not in operator with lists
print("2. not in operator with lists")
if 10 not in numbers:
    print("   ✓ 10 not in [1,2,3,4,5]")
if 3 not in numbers:
    print("   ✗ Should find 3")
else:
    print("   ✓ 3 is in list")
print("")

# Test 3: in operator with strings
print("3. in operator with strings")
text = "hello world"
if "world" in text:
    print("   ✓ 'world' in 'hello world'")
if "hello" in text:
    print("   ✓ 'hello' in 'hello world'")
if "xyz" in text:
    print("   ✗ Should not find 'xyz'")
else:
    print("   ✓ 'xyz' not in string")
print("")

# Test 4: not in operator with strings
print("4. not in operator with strings")
if "xyz" not in text:
    print("   ✓ 'xyz' not in 'hello world'")
if "world" not in text:
    print("   ✗ Should find 'world'")
else:
    print("   ✓ 'world' is in string")
print("")

# Test 5: in operator with dictionaries
print("5. in operator with dictionaries")
data = {"name": "Alice", "age": "30", "city": "NYC"}
if "name" in data:
    print("   ✓ 'name' key exists")
if "age" in data:
    print("   ✓ 'age' key exists")
if "email" in data:
    print("   ✗ Should not find 'email'")
else:
    print("   ✓ 'email' key does not exist")
print("")

# Test 6: not in operator with dictionaries
print("6. not in operator with dictionaries")
if "email" not in data:
    print("   ✓ 'email' not in dict")
if "name" not in data:
    print("   ✗ Should find 'name'")
else:
    print("   ✓ 'name' is in dict")
print("")

# Test 7: in operator in loops
print("7. in operator in loops")
found_count = 0
search_items = [2, 4, 6]
for item in search_items:
    if item in numbers:
        found_count = found_count + 1
print("   Found", found_count, "items from search list")
print("")

# Test 8: Practical use case
print("8. Practical use case - validation")
valid_statuses = [200, 201, 204]
test_status = 200

if test_status in valid_statuses:
    print("   ✓ Status", test_status, "is valid")
else:
    print("   ✗ Status is invalid")

test_status = 404
if test_status not in valid_statuses:
    print("   ✓ Status", test_status, "is invalid")
else:
    print("   ✗ Status should be invalid")
print("")

print("=== All Membership Operator Tests Complete ===")
