# Test random library - comprehensive
import random

# Test seed for reproducibility
random.seed(42)
a = random.random()
random.seed(42)
b = random.random()
a == b

# Test randint range
for i in range(10):
    n = random.randint(1, 10)
    n >= 1 and n <= 10

# Test uniform range
for i in range(10):
    f = random.uniform(0.0, 1.0)
    f >= 0.0 and f <= 1.0

# Test choice
items = ["a", "b", "c"]
c = random.choice(items)
c in items

# Test sample
s = random.sample([1, 2, 3, 4, 5], 3)
len(s) == 3

# Test choice with string
text = "hello"
c_str = random.choice(text)
c_str in text

# Test shuffle functionality
original = [1, 2, 3, 4, 5]
shuffled = [1, 2, 3, 4, 5]
random.shuffle(shuffled)
len(shuffled) == 5
# Check that all elements are still there
shuffled.sort()
result = shuffled == original
print("All random tests passed!")
result
