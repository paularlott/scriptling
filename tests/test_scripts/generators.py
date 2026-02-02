failures = 0

# Generator in function
text = "this is a test"
result = ' '.join(word.upper() for word in text.split())
if result != "THIS IS A TEST":
    failures += 1

# Generator with condition
numbers = [1, 2, 3, 4, 5]
evens = [x for x in (x for x in numbers if x % 2 == 0)]
if len(evens) != 2 or evens[0] != 2:
    failures += 1

failures == 0