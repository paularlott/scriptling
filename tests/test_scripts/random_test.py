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

# Test random.choices with weights
random.seed(42)
colors = ["red", "green", "blue"]
result = random.choices(colors, weights=[5.0, 3.0, 1.0], k=10)
assert len(result) == 10
for c in result:
    assert c in colors

# Test random.choices without weights (uniform)
result = random.choices(colors, k=5)
assert len(result) == 5
for c in result:
    assert c in colors

# Test random.choices k=1 default
result = random.choices(colors, weights=[1.0, 1.0, 1.0])
assert len(result) == 1
assert result[0] in colors

# Test random.betavariate - values in [0, 1]
random.seed(42)
for i in range(20):
    v = random.betavariate(2.0, 5.0)
    assert v >= 0.0 and v <= 1.0

# Test random.gammavariate - positive values
for i in range(20):
    v = random.gammavariate(2.0, 1.0)
    assert v >= 0.0

# Test random.triangular
for i in range(20):
    v = random.triangular(0.0, 10.0)
    assert v >= 0.0 and v <= 10.0
v_mode = random.triangular(0.0, 10.0, 5.0)
assert v_mode >= 0.0 and v_mode <= 10.0

# Test random.paretovariate - positive values
for i in range(20):
    v = random.paretovariate(1.5)
    assert v >= 1.0

# Test random.weibullvariate - positive values
for i in range(20):
    v = random.weibullvariate(1.0, 1.5)
    assert v >= 0.0