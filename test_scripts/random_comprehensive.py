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

# Test shuffle
lst = [1, 2, 3, 4, 5]
random.shuffle(lst)
len(lst) == 5
