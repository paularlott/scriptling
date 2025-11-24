# Test break statement
print("Testing break:")
i = 0
while i < 10:
    if i == 5:
        break
    print(i)
    i += 1
print("Broke at 5")

# Test continue statement
print("\nTesting continue:")
for num in [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]:
    if num % 2 == 0:
        continue
    print("Odd:", num)

# Test break in for loop
print("\nTesting break in for loop:")
for item in ["a", "b", "c", "d", "e"]:
    if item == "d":
        break
    print(item)

# Test pass statement
print("\nTesting pass:")
for i in [1, 2, 3]:
    if i == 2:
        pass
    else:
        print(i)

print("\nAll loop control tests passed!")