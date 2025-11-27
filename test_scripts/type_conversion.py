failures = 0

# str()
if str(123) != "123":
    failures += 1
if str(45.67) != "45.67":
    failures += 1

# int()
if int("456") != 456:
    failures += 1
if int(78.9) != 78:
    failures += 1

# float()
if float("12.34") != 12.34:
    failures += 1
if float(56) != 56.0:
    failures += 1

# len()
if len([1,2,3]) != 3:
    failures += 1
if len("hello") != 5:
    failures += 1

failures == 0