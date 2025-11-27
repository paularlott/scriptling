failures = 0

# Basic
numbers = [i for i in range(5)]
if len(numbers) != 5 or numbers[0] != 0 or numbers[4] != 4:
    failures += 1

# With expression
squares = [i * i for i in range(5)]
if len(squares) != 5 or squares[0] != 0 or squares[4] != 16:
    failures += 1

# With condition
evens = [i for i in range(10) if i % 2 == 0]
if len(evens) != 5 or evens[0] != 0 or evens[4] != 8:
    failures += 1

# From list
original = [1, 2, 3, 4, 5]
doubled = [x * 2 for x in original]
if len(doubled) != 5 or doubled[0] != 2 or doubled[4] != 10:
    failures += 1

# String
text = "hello"
chars = [c for c in text]
if len(chars) != 5 or chars[0] != "h" or chars[4] != "o":
    failures += 1

failures == 0