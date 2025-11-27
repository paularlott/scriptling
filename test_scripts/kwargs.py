failures = 0

def add(a, b=10, c=20):
    return a + b + c

if add(5) != 35:
    failures += 1
if add(5, c=30) != 45:
    failures += 1
if add(a=1, b=2, c=3) != 6:
    failures += 1

def greet(name, greeting="Hello"):
    return greeting + ", " + name + "!"

if greet("World") != "Hello, World!":
    failures += 1
if greet(name="Scriptling", greeting="Hi") != "Hi, Scriptling!":
    failures += 1

failures == 0