failures = 0

# +=
x = 10
x += 5
if x != 15:
    failures += 1

# -=
x = 10
x -= 3
if x != 7:
    failures += 1

# *=
x = 10
x *= 2
if x != 20:
    failures += 1

# /=
x = 10
x /= 2
if x != 5:
    failures += 1

# String +=
text = "Hello"
text += " World"
if text != "Hello World":
    failures += 1

failures == 0