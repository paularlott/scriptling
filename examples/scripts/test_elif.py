# Test: elif statement

print("=== Testing Elif ===")

# Simple elif
score = 85
if score >= 90:
    print("Grade: A")
elif score >= 80:
    print("Grade: B")
elif score >= 70:
    print("Grade: C")
else:
    print("Grade: F")

# Multiple elif chains
temp = 25
if temp < 0:
    print("Freezing")
elif temp < 10:
    print("Cold")
elif temp < 20:
    print("Cool")
elif temp < 30:
    print("Warm")
else:
    print("Hot")

# Nested elif
x = 15
y = 20
if x > 20:
    print("x is large")
elif x > 10:
    if y > 15:
        print("x is medium and y is large")
    else:
        print("x is medium and y is small")
else:
    print("x is small")

print("✓ All elif tests passed")
