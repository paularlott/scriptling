# for/else: else runs when loop completes without break
result = ""
for i in range(3):
    result += str(i)
else:
    result += "done"
assert result == "012done"

# for/else: else does NOT run when break occurs
result = ""
for i in range(5):
    if i == 2:
        break
    result += str(i)
else:
    result += "done"
assert result == "01"

# for/else: else runs on empty iterable
result = "empty"
for i in []:
    result = "not empty"
else:
    result += "_else"
assert result == "empty_else"

# while/else: else runs when condition becomes false
n = 3
result = ""
while n > 0:
    result += str(n)
    n -= 1
else:
    result += "done"
assert result == "321done"

# while/else: else does NOT run when break occurs
n = 5
result = ""
while n > 0:
    if n == 3:
        break
    result += str(n)
    n -= 1
else:
    result += "done"
assert result == "54"

# for/else: common search pattern
def find_prime(numbers):
    for n in numbers:
        for i in range(2, n):
            if n % i == 0:
                break
        else:
            return n
    return None

primes = [4, 6, 7, 8, 9]
assert find_prime(primes) == 7

# while/else: loop that never executes
x = 0
while x > 10:
    x += 1
else:
    x = 99
assert x == 99

print("All for/else and while/else tests passed!")
