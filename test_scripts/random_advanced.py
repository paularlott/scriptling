# Test new random library functions
import random

# Seed for reproducibility
random.seed(42)

# Test randrange with 1 argument
result = random.randrange(10)
assert result >= 0 and result < 10, "randrange(10)"

# Test randrange with 2 arguments
result = random.randrange(5, 15)
assert result >= 5 and result < 15, "randrange(5, 15)"

# Test randrange with step
result = random.randrange(0, 10, 2)
assert result >= 0 and result < 10 and result % 2 == 0, "randrange(0, 10, 2)"

# Test gauss - generate several values and check they're distributed
total = 0
for i in range(100):
    total = total + random.gauss(0, 1)
mean = total / 100
# Mean should be close to 0 for many samples
assert mean > -1 and mean < 1, "gauss mean should be near 0"

# Test normalvariate
result = random.normalvariate(100, 15)
# Result should be in a reasonable range
assert result > 0 and result < 200, "normalvariate result in range"

# Test expovariate
result = random.expovariate(1.0)
assert result >= 0, "expovariate should be non-negative"

print("All new random tests passed!")

# Return true for test framework
True
