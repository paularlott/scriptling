import random

# Weighted dice rolls
faces = [1, 2, 3, 4, 5, 6]
weights = [1, 1, 1, 1, 1, 2]  # 6 is twice as likely

rolls = random.choices(faces, weights=weights, k=20)
print(f"20 weighted dice rolls: {rolls}")

# Simulate 1000 dice rolls and count distribution
counts = {}
for i in range(1000):
    r = random.choices(faces, weights=weights, k=1)[0]
    counts[r] = counts.get(r, 0) + 1

print("\nDistribution over 1000 rolls:")
for face in sorted(list(counts.keys())):
    bar = "#" * (counts[face] // 5)
    print(f"  {face}: {counts[face]:4d} {bar}")

# Monte Carlo estimate of pi
random.seed(42)
inside = 0
total = 10000
for i in range(total):
    x = random.uniform(-1, 1)
    y = random.uniform(-1, 1)
    if x*x + y*y <= 1:
        inside += 1
pi_estimate = 4.0 * inside / total
print(f"\nMonte Carlo pi estimate ({total} samples): {pi_estimate:.4f}")
print(f"Actual pi: {3.14159:.4f}")

# Sampling from different distributions
print("\nDistribution samples:")
print(f"  Gaussian(0, 1):     {[round(random.gauss(0, 1), 2) for _ in range(5)]}")
print(f"  Beta(2, 5):         {[round(random.betavariate(2, 5), 3) for _ in range(5)]}")
print(f"  Gamma(2, 2):        {[round(random.gammavariate(2, 2), 2) for _ in range(5)]}")
print(f"  Triangular(0,10,5): {[round(random.triangular(0, 10, 5), 2) for _ in range(5)]}")
print(f"  Pareto(1.5):        {[round(random.paretovariate(1.5), 2) for _ in range(5)]}")
print(f"  Weibull(1, 1.5):    {[round(random.weibullvariate(1.0, 1.5), 2) for _ in range(5)]}")

# Shuffle and deal cards
suits = ["S", "H", "D", "C"]
ranks = ["A", "2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K"]
deck = [f"{r}{s}" for s in suits for r in ranks]
random.shuffle(deck)

hands = [[], [], [], []]
for i, card in enumerate(deck):
    hands[i % 4].append(card)

print(f"\nDealt 4 hands of {len(hands[0])} cards:")
for i, hand in enumerate(hands):
    print(f"  Player {i+1}: {hand}")
