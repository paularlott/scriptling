# Test: Random library

import random

print("=== Testing Random Library ===")

# random() - returns float between 0 and 1
r = random.random()
print(f"random(): {r}")

# randint(a, b) - returns random integer between a and b
r = random.randint(1, 10)
print(f"randint(1, 10): {r}")

# choice(list) - returns random element from list
fruits = ["apple", "banana", "cherry", "date"]
r = random.choice(fruits)
print(f"choice(fruits): {r}")

# Multiple random numbers
print("Five random integers:")
for i in range(5):
    r = random.randint(1, 100)
    print(f"  {r}")

print("✓ All random library tests passed")
