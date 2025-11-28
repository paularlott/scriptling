# Test new math library functions
import math

# Test isnan
assert math.isnan(math.nan) == True, "isnan(nan)"
assert math.isnan(1.0) == False, "isnan(1.0)"
assert math.isnan(0) == False, "isnan(0)"

# Test isinf
assert math.isinf(math.inf) == True, "isinf(inf)"
assert math.isinf(-math.inf) == True, "isinf(-inf)"
assert math.isinf(1.0) == False, "isinf(1.0)"

# Test isfinite
assert math.isfinite(1.0) == True, "isfinite(1.0)"
assert math.isfinite(math.inf) == False, "isfinite(inf)"
assert math.isfinite(math.nan) == False, "isfinite(nan)"

# Test copysign
assert math.copysign(1.0, -1.0) == -1.0, "copysign positive to negative"
assert math.copysign(-1.0, 1.0) == 1.0, "copysign negative to positive"

# Test trunc
assert math.trunc(3.7) == 3, "trunc(3.7)"
assert math.trunc(-3.7) == -3, "trunc(-3.7)"
assert math.trunc(5) == 5, "trunc(5)"

# Test log10
result = math.log10(100)
assert result > 1.99 and result < 2.01, "log10(100) should be ~2"

# Test log2
result = math.log2(8)
assert result > 2.99 and result < 3.01, "log2(8) should be ~3"

# Test hypot
result = math.hypot(3, 4)
assert result > 4.99 and result < 5.01, "hypot(3,4) should be 5"

# Test asin
result = math.asin(0)
assert result > -0.01 and result < 0.01, "asin(0) should be ~0"

# Test acos
result = math.acos(1)
assert result > -0.01 and result < 0.01, "acos(1) should be ~0"

# Test atan
result = math.atan(0)
assert result > -0.01 and result < 0.01, "atan(0) should be ~0"

# Test atan2
result = math.atan2(1, 1)
assert result > 0.78 and result < 0.79, "atan2(1,1) should be ~pi/4"

print("All new math tests passed!")

# Return true for test framework
True
