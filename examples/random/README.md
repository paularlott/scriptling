# Random Library Examples

Examples of using the `random` library for random number generation and sampling.

## Functions

### Basics
| Function | Description |
|----------|-------------|
| `random.seed([a])` | Initialize the RNG (omit for time-based seed) |
| `random.random()` | Float in [0.0, 1.0) |
| `random.randint(min, max)` | Integer in [min, max] |
| `random.uniform(a, b)` | Float in [a, b] |

### Sampling
| Function | Description |
|----------|-------------|
| `random.choice(seq)` | Single random element from list or string |
| `random.choices(pop, weights=None, k=1)` | Weighted sampling with replacement |
| `random.sample(pop, k)` | k unique random elements (without replacement) |
| `random.shuffle(list)` | Shuffle list in-place |

### Ranges
| Function | Description |
|----------|-------------|
| `random.randrange(stop)` | Integer in [0, stop) |
| `random.randrange(start, stop[, step])` | Integer from range |

### Distributions
| Function | Description |
|----------|-------------|
| `random.gauss(mu, sigma)` | Normal (Gaussian) distribution |
| `random.betavariate(alpha, beta)` | Beta distribution, result in [0, 1] |
| `random.gammavariate(alpha, beta)` | Gamma distribution |
| `random.triangular(low, high[, mode])` | Triangular distribution |
| `random.paretovariate(alpha)` | Pareto distribution |
| `random.weibullvariate(alpha, beta)` | Weibull distribution |
| `random.expovariate(lambd)` | Exponential distribution |

## Examples

- **distributions.py** - Weighted dice rolls, Monte Carlo pi estimation, distribution sampling, card dealing

## Quick Start

```python
import random

random.seed(42)

# Basics
r = random.random()          # [0, 1)
n = random.randint(1, 10)    # 1..10
f = random.uniform(0, 100)   # float in [0, 100]

# Weighted sampling
colors = ["red", "green", "blue"]
picks = random.choices(colors, weights=[5, 3, 1], k=10)

# Distributions
x = random.gauss(0, 1)       # Normal
b = random.betavariate(2, 5) # Beta in [0, 1]
g = random.gammavariate(2, 1) # Gamma

# Shuffle
items = [1, 2, 3, 4, 5]
random.shuffle(items)
```
