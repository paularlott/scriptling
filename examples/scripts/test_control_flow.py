# Test: Control flow (if/elif/else)

print("=== Testing Control Flow ===")

# Simple if
x = 10
if x > 5:
    print("x is greater than 5")

# If-else
x = 3
if x > 5:
    print("x is greater than 5")
else:
    print("x is not greater than 5")

# If-elif-else
score = 75
if score >= 90:
    print("Grade: A")
elif score >= 80:
    print("Grade: B")
elif score >= 70:
    print("Grade: C")
else:
    print("Grade: F")

# Nested if
a = 10
b = 20
if a > 5:
    if b > 15:
        print("Both conditions true")
    else:
        print("First true, second false")
else:
    print("First condition false")

# Multiple conditions
age = 25
has_license = True
if age >= 18 and has_license:
    print("Can drive")

# Pass statement
if False:
    pass
else:
    print("Pass statement works")

print("✓ All control flow tests passed")
