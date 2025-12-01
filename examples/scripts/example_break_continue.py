# Example: break and continue statements

print("Break and Continue")

# Test break
print("Test break:")
for i in range(10):
    if i == 5:
        break
    print(f"  i = {i}")

# Test continue
print("Test continue (skip even numbers):")
for i in range(10):
    if i % 2 == 0:
        continue
    print(f"  i = {i}")

# Test break in while
print("Test break in while:")
count = 0
while True:
    if count >= 3:
        break
    print(f"  count = {count}")
    count = count + 1

# Test continue in while
print("Test continue in while:")
count = 0
while count < 5:
    count = count + 1
    if count == 3:
        continue
    print(f"  count = {count}")

# Nested loops with break
print("Test nested loops with break:")
for i in range(3):
    for j in range(3):
        if j == 2:
            break
        print(f"  i={i}, j={j}")

