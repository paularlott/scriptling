failures = 0

# Basic lambda
double = lambda x: x * 2
if double(5) != 10:
    failures += 1

# Multiple params
add = lambda a, b: a + b
if add(3, 7) != 10:
    failures += 1

# With list
numbers = [1, 2, 3]
square = lambda x: x * x
squares = [square(n) for n in numbers]
if len(squares) != 3 or squares[0] != 1 or squares[2] != 9:
    failures += 1

# Boolean lambda
is_even = lambda n: n % 2 == 0
if not is_even(4) or is_even(7):
    failures += 1

failures == 0