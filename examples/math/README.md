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
| `math.array(data)` | Create an efficient FloatArray from a list |
| `math.shape(a)` | Return the shape of a FloatArray as a list of ints |

### Activation / Probability
| Function | Description |
|----------|-------------|
| `math.tanh(x)` | Hyperbolic tangent |
| `math.softmax(x)` | Numerically stable softmax → probability distribution |

### FloatArray

Efficient numerical array type that avoids per-element boxing overhead. Supports 1D and 2D arrays.

```python
import math

# Create arrays
a = math.array([1.0, 2.0, 3.0])                    # 1D
m = math.array([[1.0, 2.0], [3.0, 4.0]])            # 2D

# Indexing and slicing
a[0]           # 1.0
m[0]           # [1.0, 2.0] (1D FloatArray)
m[0][1]        # 2.0
m[0:2]         # 2D FloatArray slice

# Assignment
a[0] = 10.0
m[0][1] = 9.0
m[1] = [5.0, 6.0]

# Shape and length
math.shape(m)  # [2, 2]
len(m)         # 2 (rows)
len(a)         # 3 (elements)

# Iteration
for v in a:          # Float values
for row in m:        # 1D FloatArray rows

# Works with math operations
math.matmul(m, m)
math.transpose(m)
math.softmax(a)
math.dot(a, a)
math.mat_add(m, m)

# Works with built-in functions
sum(a), min(a), max(a)
list(reversed(a))
list(enumerate(a))
```

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

# FloatArray for efficient operations
a = math.array([1.0, 2.0, 3.0])
m = math.array([[1.0, 2.0], [3.0, 4.0]])
print(math.shape(m))  # [2, 2]
print(math.matmul(m, m))  # 2D FloatArray

# Softmax
probs = math.softmax([2.0, 1.0, 0.1])

# Combinatorics
math.comb(5, 2)   # 10
math.perm(5, 2)   # 20
math.prod([2, 3])  # 6

# Distance
math.dist([0, 0], [3, 4])  # 5.0
```
