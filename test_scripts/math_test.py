import math

# Test math.fabs (Python compatible - always returns float)
assert math.fabs(-42) == 42.0
assert math.fabs(42) == 42.0
assert math.fabs(-3.14) == 3.14
assert math.fabs(3.14) == 3.14

# Test builtins (abs, min, max, round)
assert abs(-42) == 42
assert abs(42) == 42
assert abs(-3.14) == 3.14
assert min(1, 2, 3) == 1
assert max(1, 2, 3) == 3
assert round(3.5) == 4
assert round(3.2) == 3

# From math_basic
assert math.pow(2, 8) == 256.0
assert math.sqrt(16) == 4.0

# From math_advanced
assert math.isnan(math.nan) == True
assert math.isnan(1.0) == False
assert math.isnan(0) == False
assert math.isinf(math.inf) == True
assert math.isinf(-math.inf) == True
assert math.isinf(1.0) == False
assert math.isfinite(1.0) == True
assert math.isfinite(math.inf) == False
assert math.isfinite(math.nan) == False
assert math.copysign(1.0, -1.0) == -1.0
assert math.copysign(-1.0, 1.0) == 1.0
assert math.trunc(3.7) == 3
assert math.trunc(-3.7) == -3
assert math.trunc(5) == 5
result = math.log10(100)
assert result > 1.99 and result < 2.01
result = math.log2(8)
assert result > 2.99 and result < 3.01
result = math.hypot(3, 4)
assert result > 4.99 and result < 5.01
result = math.asin(0)
assert result > -0.01 and result < 0.01
result = math.acos(1)
assert result > -0.01 and result < 0.01
result = math.atan(0)
assert result > -0.01 and result < 0.01

# From math_comprehensive
assert math.pi > 3.14 and math.pi < 3.15
assert math.e > 2.71 and math.e < 2.72
assert math.floor(3.7) == 3
assert math.ceil(3.2) == 4
assert math.floor(-3.7) == -4
assert math.ceil(-3.2) == -3
assert math.log(math.e) > 0.99 and math.log(math.e) < 1.01
assert math.exp(0) == 1.0
assert math.exp(1) > 2.71 and math.exp(1) < 2.72
assert math.sin(0) == 0.0
assert math.cos(0) == 1.0
assert math.tan(0) == 0.0
assert math.pow(2, 3) == 8.0
assert math.sqrt(16) == 4.0
assert math.degrees(math.pi) > 179 and math.degrees(math.pi) < 181
assert math.radians(180) > 3.14 and math.radians(180) < 3.15
assert math.gcd(12, 8) == 4
assert math.gcd(15, 25) == 5
assert math.factorial(5) == 120
assert math.factorial(0) == 1

# From math_sqrt
result = math.sqrt(16)
assert result == 4.0

# Basic arithmetic
x = 10
y = 5
sum_val = x + y
diff = x - y
prod = x * y
quot = x / y
mod = x % y
assert sum_val == 15
assert diff == 5
assert prod == 50
assert quot == 2.0
assert mod == 0

# Test pow() and divmod() builtins
assert pow(2, 3) == 8
assert pow(2, 10) == 1024
assert pow(3, 3) == 27
assert pow(10, 0) == 1
assert pow(5, -1) == 0.2

assert pow(2, 10, 1000) == 24
assert pow(3, 4, 5) == 1
assert pow(7, 3, 11) == 2

dm1 = divmod(17, 5)
assert dm1[0] == 3 and dm1[1] == 2
dm2 = divmod(10, 3)
assert dm2[0] == 3 and dm2[1] == 1
dm3 = divmod(20, 4)
assert dm3[0] == 5 and dm3[1] == 0

# Test floor division operator //
result = 7 // 2
assert result == 3

result = -7 // 2
assert result == -3

result = 10 // 3
assert result == 3

# Float floor division
result = 7.5 // 2
assert result == 3.0