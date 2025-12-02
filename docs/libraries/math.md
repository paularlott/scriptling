# Math Library

Mathematical functions and constants.

## Functions

### math.sqrt(x)

Returns the square root of x.

**Parameters:**
- `x`: Number (integer or float)

**Returns:** Float

**Example:**
```python
import math
result = math.sqrt(16)  # 4.0
```

### math.pow(base, exp)

Returns base raised to the power of exp (base^exp).

**Parameters:**
- `base`: Base number
- `exp`: Exponent

**Returns:** Float

**Example:**
```python
import math
result = math.pow(2, 8)  # 256.0
```

### math.fabs(x)

Returns the absolute value of x as a float.

**Parameters:**
- `x`: Number (integer or float)

**Returns:** Float (always returns floating-point)

**Example:**
```python
import math
result = math.fabs(-5)    # 5.0
result = math.fabs(-3.14) # 3.14
```

> **Note:** For absolute value that preserves integer type, use the builtin `abs()` function instead.

### math.floor(x)

Rounds x down to the nearest integer.

**Parameters:**
- `x`: Number

**Returns:** Integer

**Example:**
```python
import math
result = math.floor(3.7)  # 3
```

### math.ceil(x)

Rounds x up to the nearest integer.

**Parameters:**
- `x`: Number

**Returns:** Integer

**Example:**
```python
import math
result = math.ceil(3.2)  # 4
```

> **Note:** For rounding to nearest integer, use the builtin `round()` function.
> For min/max values, use the builtin `min()` and `max()` functions.

### math.sin(x)

Returns the sine of x (in radians).

**Parameters:**
- `x`: Angle in radians

**Returns:** Float

**Example:**
```python
import math
result = math.sin(0)  # 0.0
result = math.sin(math.pi / 2)  # 1.0
```

### math.cos(x)

Returns the cosine of x (in radians).

**Parameters:**
- `x`: Angle in radians

**Returns:** Float

**Example:**
```python
import math
result = math.cos(0)  # 1.0
result = math.cos(math.pi)  # -1.0
```

### math.tan(x)

Returns the tangent of x (in radians).

**Parameters:**
- `x`: Angle in radians

**Returns:** Float

**Example:**
```python
import math
result = math.tan(0)  # 0.0
result = math.tan(math.pi / 4)  # 1.0
```

### math.log(x)

Returns the natural logarithm (base e) of x.

**Parameters:**
- `x`: Number (must be > 0)

**Returns:** Float

**Example:**
```python
import math
result = math.log(1)  # 0.0
result = math.log(math.e)  # 1.0
```

### math.exp(x)

Returns e raised to the power of x (e^x).

**Parameters:**
- `x`: Number

**Returns:** Float

**Example:**
```python
import math
result = math.exp(0)  # 1.0
result = math.exp(1)  # 2.718281828459045
```

### math.degrees(x)

Converts angle x from radians to degrees.

**Parameters:**
- `x`: Angle in radians

**Returns:** Float

**Example:**
```python
import math
result = math.degrees(math.pi)  # 180.0
result = math.degrees(math.pi / 2)  # 90.0
```

### math.radians(x)

Converts angle x from degrees to radians.

**Parameters:**
- `x`: Angle in degrees

**Returns:** Float

**Example:**
```python
import math
result = math.radians(180)  # 3.141592653589793
result = math.radians(90)   # 1.5707963267948966
```

### math.fmod(x, y)

Returns the floating-point remainder of x divided by y.

**Parameters:**
- `x`: Dividend
- `y`: Divisor (cannot be 0)

**Returns:** Float

**Example:**
```python
import math
result = math.fmod(5.5, 2.0)  # 1.5
result = math.fmod(7.0, 3.0)  # 1.0
```

### math.gcd(a, b)

Returns the greatest common divisor of integers a and b.

**Parameters:**
- `a`: Integer
- `b`: Integer

**Returns:** Integer

**Example:**
```python
import math
result = math.gcd(48, 18)  # 6
result = math.gcd(100, 75) # 25
```

### math.factorial(n)

Returns the factorial of n (n!).

**Parameters:**
- `n`: Non-negative integer (0 ≤ n ≤ 20)

**Returns:** Integer

**Example:**
```python
import math
result = math.factorial(5)  # 120
result = math.factorial(0)  # 1
```

## Constants

### math.pi

The mathematical constant π (pi).

**Value:** Float (3.141592653589793)

**Example:**
```python
import math
pi = math.pi  # 3.141592653589793
```

### math.e

The mathematical constant e (Euler's number).

**Value:** Float (2.718281828459045)

**Example:**
```python
import math
e = math.e  # 2.718281828459045
```

## Usage Example

```python
import math

# Basic operations
result = math.sqrt(16)      # 4.0
power = math.pow(2, 8)      # 256.0
absolute = math.fabs(-5)    # 5.0 (float)

# For integer-preserving abs, use builtin:
int_abs = abs(-5)           # 5

# Rounding
floor_val = math.floor(3.7) # 3
ceil_val = math.ceil(3.2)   # 4
round_val = round(3.5)      # 4 (use builtin round)

# Min/Max (use builtins)
minimum = min(3, 1, 4, 1, 5)  # 1
maximum = max(3, 1, 4, 1, 5)  # 5

# Trigonometric functions
sin_val = math.sin(0)       # 0.0
cos_val = math.cos(0)       # 1.0
tan_val = math.tan(0)       # 0.0

# Logarithmic and exponential
log_val = math.log(1)       # 0.0
exp_val = math.exp(1)       # 2.718281828459045

# Angle conversion
degrees_val = math.degrees(math.pi)  # 180.0
radians_val = math.radians(180)        # 3.141592653589793

# Modular arithmetic
mod_val = math.fmod(5.5, 2.0)  # 1.5
gcd_val = math.gcd(48, 18)     # 6
fact_val = math.factorial(5)   # 120

# Constants
pi = math.pi  # 3.141592653589793
e = math.e    # 2.718281828459045

# Calculate circle area
radius = 5
area = math.pi * math.pow(radius, 2)
print("Area: " + str(area))  # Area: 78.53981633974483

# Calculate hypotenuse using Pythagoras
a = 3
b = 4
hypotenuse = math.sqrt(math.pow(a, 2) + math.pow(b, 2))
print("Hypotenuse: " + str(hypotenuse))  # Hypotenuse: 5.0
```