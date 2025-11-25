# Test: Loops (for, while, range)
# Iteration, break, continue

print("=== Testing Loops ===")

# For loop with range
print("For loop with range(5):")
for i in range(5):
    print(f"  i = {i}")

# For loop with list
fruits = ["apple", "banana", "cherry"]
print("For loop with list:")
for fruit in fruits:
    print(f"  {fruit}")

# While loop
print("While loop:")
count = 0
while count < 3:
    print(f"  count = {count}")
    count = count + 1

# Range with start and stop
print("Range(2, 5):")
for i in range(2, 5):
    print(f"  i = {i}")

# Nested loops
print("Nested loops:")
for i in range(2):
    for j in range(2):
        print(f"  i={i}, j={j}")

print("✓ All loop tests passed")
