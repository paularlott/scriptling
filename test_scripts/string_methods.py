# Test string methods
failures = 0

# find
s = "hello world"
if s.find("world") != 6:
    failures += 1
if s.find("xyz") != -1:
    failures += 1

# index
if s.index("o") != 4:
    failures += 1

# count
if s.count("o") != 2:
    failures += 1
if s.count("l") != 3:
    failures += 1

# format
template = "Hello, {}!"
result = template.format("World")
if result != "Hello, World!":
    failures += 1

template2 = "{} + {} = {}"
result2 = template2.format(1, 2, 3)
if result2 != "1 + 2 = 3":
    failures += 1

# isdigit
if not "123".isdigit():
    failures += 1
if "12a".isdigit():
    failures += 1

# isalpha
if not "abc".isalpha():
    failures += 1
if "ab1".isalpha():
    failures += 1

# isalnum
if not "abc123".isalnum():
    failures += 1
if "abc 123".isalnum():
    failures += 1

# isspace
if not "   ".isspace():
    failures += 1
if " a ".isspace():
    failures += 1

# isupper / islower
if not "HELLO".isupper():
    failures += 1
if not "hello".islower():
    failures += 1

# zfill
if "42".zfill(5) != "00042":
    failures += 1
if "-42".zfill(5) != "-0042":
    failures += 1

# center
if "hi".center(6) != "  hi  ":
    failures += 1

# ljust / rjust
if "hi".ljust(5) != "hi   ":
    failures += 1
if "hi".rjust(5) != "   hi":
    failures += 1

failures == 0
