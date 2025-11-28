# Test math library - comprehensive
import math

# Test constants
math.pi > 3.14 and math.pi < 3.15
math.e > 2.71 and math.e < 2.72

# Test floor and ceil
math.floor(3.7) == 3
math.ceil(3.2) == 4
math.floor(-3.7) == -4
math.ceil(-3.2) == -3

# Test log and exp
math.log(math.e) > 0.99 and math.log(math.e) < 1.01
math.exp(0) == 1.0
math.exp(1) > 2.71 and math.exp(1) < 2.72

# Test trig functions
math.sin(0) == 0.0
math.cos(0) == 1.0
math.tan(0) == 0.0

# Test min/max
math.min(1, 2, 3) == 1
math.max(1, 2, 3) == 3

# Test abs
math.abs(-5) == 5
math.abs(5) == 5

# Test pow and sqrt
math.pow(2, 3) == 8.0
math.sqrt(16) == 4.0

# Test round
math.round(3.7) == 4
math.round(3.2) == 3

# Test degrees and radians
math.degrees(math.pi) > 179 and math.degrees(math.pi) < 181
math.radians(180) > 3.14 and math.radians(180) < 3.15

# Test gcd
math.gcd(12, 8) == 4
math.gcd(15, 25) == 5

# Test factorial
math.factorial(5) == 120
math.factorial(0) == 1
