# Test math library
# Priority 2 feature

import math

print("=== Math Library Test ===")
print("")

# Test 1: sqrt
print("1. Square root")
print("   sqrt(16) =", math.sqrt(16))
print("   sqrt(25) =", math.sqrt(25))
print("   sqrt(2) =", math.sqrt(2))
print("")

# Test 2: pow
print("2. Power")
print("   pow(2, 3) =", math.pow(2, 3))
print("   pow(2, 8) =", math.pow(2, 8))
print("   pow(5, 2) =", math.pow(5, 2))
print("")

# Test 3: abs
print("3. Absolute value")
print("   abs(-5) =", math.abs(-5))
print("   abs(5) =", math.abs(5))
print("   abs(-3.14) =", math.abs(-3.14))
print("")

# Test 4: floor
print("4. Floor")
print("   floor(3.7) =", math.floor(3.7))
print("   floor(3.2) =", math.floor(3.2))
print("   floor(-2.5) =", math.floor(-2.5))
print("")

# Test 5: ceil
print("5. Ceiling")
print("   ceil(3.2) =", math.ceil(3.2))
print("   ceil(3.7) =", math.ceil(3.7))
print("   ceil(-2.5) =", math.ceil(-2.5))
print("")

# Test 6: round
print("6. Round")
print("   round(3.5) =", math.round(3.5))
print("   round(3.4) =", math.round(3.4))
print("   round(3.6) =", math.round(3.6))
print("")

# Test 7: min
print("7. Minimum")
print("   min(1, 5, 3) =", math.min(1, 5, 3))
print("   min(10, 20) =", math.min(10, 20))
print("   min(-5, 0, 5) =", math.min(-5, 0, 5))
print("")

# Test 8: max
print("8. Maximum")
print("   max(1, 5, 3) =", math.max(1, 5, 3))
print("   max(10, 20) =", math.max(10, 20))
print("   max(-5, 0, 5) =", math.max(-5, 0, 5))
print("")

# Test 9: Constants
print("9. Constants")
pi = math.pi()
e = math.e()
print("   pi =", pi)
print("   e =", e)
print("")

# Test 10: Practical calculations
print("10. Practical calculations")

# Circle area
radius = 5
area = pi * math.pow(radius, 2)
print("   Circle area (r=5) =", area)

# Circle circumference
circumference = 2 * pi * radius
print("   Circle circumference (r=5) =", circumference)

# Distance formula
x1 = 0
y1 = 0
x2 = 3
y2 = 4
dx = x2 - x1
dy = y2 - y1
distance = math.sqrt(math.pow(dx, 2) + math.pow(dy, 2))
print("   Distance (0,0) to (3,4) =", distance)

# Compound interest
principal = 1000
rate = 0.05
time = 10
amount = principal * math.pow(1 + rate, time)
print("   Compound interest result =", math.round(amount))

print("")
print("=== All Math Library Tests Complete ===")
