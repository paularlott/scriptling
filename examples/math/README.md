# Math Library Examples

Examples of using the `math` library's linear algebra and advanced functions.

## Linear Algebra

- **matrix_ops.py** - Matrix multiplication, transpose, addition, dot product, outer product
- **neural_network.py** - Neural network forward pass using matmul, tanh, softmax

## New Functions (since added)

### Linear Algebra
| Function | Description |
|----------|-------------|
| `math.dot(a, b)` | Dot product of two vectors |
| `math.matmul(a, b)` | Matrix-matrix multiply (M×K) @ (K×N) → (M×N) |
| `math.transpose(m)` | Transpose a 2D matrix |
| `math.mat_add(a, b)` | Element-wise matrix addition |

### Activation / Probability
| Function | Description |
|----------|-------------|
| `math.tanh(x)` | Hyperbolic tangent |
| `math.softmax(x)` | Numerically stable softmax → probability distribution |

### Special Functions
| Function | Description |
|----------|-------------|
| `math.erf(x)` | Error function |
| `math.erfc(x)` | Complementary error function |
| `math.gamma(x)` | Gamma function |
| `math.lgamma(x)` | Log-gamma → `[log_abs, sign]` |

### Numerics
| Function | Description |
|----------|-------------|
| `math.cbrt(x)` | Cube root |
| `math.log1p(x)` | `log(1+x)` accurate for small x |
| `math.expm1(x)` | `exp(x)-1` accurate for small x |
| `math.nextafter(x, y)` | Next float toward y |
| `math.remainder(x, y)` | IEEE 754 remainder |

### Combinatorics
| Function | Description |
|----------|-------------|
| `math.comb(n, k)` | Binomial coefficient |
| `math.perm(n[, k])` | Permutations (n! if k omitted) |
| `math.prod(iterable, start=1)` | Product of all elements |
| `math.dist(p, q)` | Euclidean distance between two points |

### Constants
| Constant | Value |
|----------|-------|
| `math.tau` | 2π |

## Quick Start

```python
import math

# Dot product
result = math.dot([1.0, 2.0, 3.0], [4.0, 5.0, 6.0])  # 32.0

# Matrix multiply
C = math.matmul([[1.0, 2.0], [3.0, 4.0]], [[5.0, 6.0], [7.0, 8.0]])

# Softmax
probs = math.softmax([2.0, 1.0, 0.1])

# Combinatorics
math.comb(5, 2)   # 10
math.perm(5, 2)   # 20
math.prod([2, 3])  # 6

# Distance
math.dist([0, 0], [3, 4])  # 5.0
```
