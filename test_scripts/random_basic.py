import random

# Test random functions
r = random.random()
assert 0 <= r and r <= 1

r_int = random.randint(1, 10)
assert 1 <= r_int and r_int <= 10

fruits = ["apple", "banana", "cherry"]
choice = random.choice(fruits)
assert choice in fruits

True