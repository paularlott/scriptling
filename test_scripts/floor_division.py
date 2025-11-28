# Test floor division operator //
failures = 0

# Integer floor division
result = 7 // 2
if result != 3:
    failures += 1

result = -7 // 2
# Go integer division truncates toward zero
if result != -3:
    failures += 1

result = 10 // 3
if result != 3:
    failures += 1

# Float floor division
result = 7.5 // 2
if result != 3.0:
    failures += 1

result = 7 // 2.0
if result != 3.0:
    failures += 1

# Augmented floor division
x = 10
x //= 3
if x != 3:
    failures += 1

failures == 0
