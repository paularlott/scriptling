import random

# Test random functions
r = random.random()
assert 0 <= r and r <= 1

r_int = random.randint(1, 10)
assert 1 <= r_int and r_int <= 10

fruits = ["apple", "banana", "cherry"]
choice = random.choice(fruits)
assert choice in fruits

# Seed for reproducibility
random.seed(42)

# Test randrange with 1 argument
result = random.randrange(10)
assert result >= 0 and result < 10

# Test randrange with 2 arguments
result = random.randrange(5, 15)
assert result >= 5 and result < 15

# Test randrange with step
result = random.randrange(0, 10, 2)
assert result >= 0 and result < 10 and result % 2 == 0

# Test gauss - generate several values and check they're distributed
total = 0
for i in range(10):
    total += random.gauss(0, 1)
assert total > -5 and total < 5  # Rough check for distribution

# Test seed for reproducibility
random.seed(42)
a = random.random()
random.seed(42)
b = random.random()
assert a == b

# Test randint range
for i in range(10):
    n = random.randint(1, 10)
    assert n >= 1 and n <= 10

# Test uniform range
for i in range(10):
    f = random.uniform(0.0, 1.0)
    assert f >= 0.0 and f <= 1.0

# Test choice
items = ["a", "b", "c"]
c = random.choice(items)
assert c in items

# Test sample
s = random.sample([1, 2, 3, 4, 5], 3)
assert len(s) == 3

# Test choice with string
text = "hello"
c_str = random.choice(text)
assert c_str in text

# Test shuffle functionality
original = [1, 2, 3, 4, 5]
shuffled = [1, 2, 3, 4, 5]
random.shuffle(shuffled)
assert len(shuffled) == 5
# Check that all elements are still there
shuffled.sort()
assert shuffled == original