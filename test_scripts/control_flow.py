# If-else
result = 0
if 10 > 5:
    result = 1
else:
    result = 0
assert result == 1

# If-elif-else
grade = ""
score = 75
if score >= 90:
    grade = "A"
elif score >= 80:
    grade = "B"
elif score >= 70:
    grade = "C"
else:
    grade = "F"
assert grade == "C"

# Nested if
msg = ""
a = 10
b = 20
if a > 5:
    if b > 15:
        msg = "both"
    else:
        msg = "first"
else:
    msg = "none"
assert msg == "both"