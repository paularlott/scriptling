# Test random library

import random

print("=== Random Library Test ===")
print("")

# Test 1: randint
print("1. randint - random integer in range")
for i in range(5):
    num = random.randint(1, 10)
    print("   Random (1-10):", num)
print("")

# Test 2: random - float between 0 and 1
print("2. random - float between 0.0 and 1.0")
for i in range(5):
    num = random.random()
    print("   Random float:", num)
print("")

# Test 3: choice - pick random element
print("3. choice - pick random element from list")
items = ["apple", "banana", "cherry", "date", "elderberry"]
for i in range(5):
    item = random.choice(items)
    print("   Random fruit:", item)
print("")

# Test 4: shuffle - shuffle list in place
print("4. shuffle - shuffle list in place")
numbers = [1, 2, 3, 4, 5]
print("   Before shuffle:", numbers)
random.shuffle(numbers)
print("   After shuffle:", numbers)
print("")

# Test 5: randint with same min/max
print("5. randint with same min/max")
same = random.randint(5, 5)
print("   randint(5, 5):", same)
print("")

# Test 6: randint with larger range
print("6. randint with larger range")
for i in range(3):
    big = random.randint(1, 100)
    print("   Random (1-100):", big)
print("")

# Test 7: choice with different types
print("7. choice with different types")
mixed = [1, "two", 3.0, True]
for i in range(4):
    item = random.choice(mixed)
    print("   Random item:", item)
print("")

# Test 8: Multiple shuffles
print("8. Multiple shuffles")
deck = [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
print("   Original:", deck)
random.shuffle(deck)
print("   Shuffle 1:", deck)
random.shuffle(deck)
print("   Shuffle 2:", deck)
print("")

# Test 9: Practical example - dice roll
print("9. Practical example - dice roll")
def roll_dice():
    return random.randint(1, 6)

print("   Rolling dice 5 times:")
for i in range(5):
    roll = roll_dice()
    print("     Roll", i + 1, ":", roll)
print("")

# Test 10: Practical example - lottery numbers
print("10. Practical example - lottery numbers")
lottery = []
for i in range(6):
    num = random.randint(1, 49)
    append(lottery, num)
print("   Lottery numbers:", lottery)
print("")

print("=== All Random Tests Complete ===")
