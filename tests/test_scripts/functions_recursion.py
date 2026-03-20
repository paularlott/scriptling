# Factorial
def factorial(n):
    if n <= 1:
        return 1
    return n * factorial(n - 1)

assert factorial(0) == 1
assert factorial(1) == 1
assert factorial(5) == 120
assert factorial(10) == 3628800

# Fibonacci
def fib(n):
    if n <= 1:
        return n
    return fib(n - 1) + fib(n - 2)

assert fib(0) == 0
assert fib(1) == 1
assert fib(10) == 55

# Mutual recursion
def is_even(n):
    if n == 0:
        return True
    return is_odd(n - 1)

def is_odd(n):
    if n == 0:
        return False
    return is_even(n - 1)

assert is_even(0) == True
assert is_even(4) == True
assert is_odd(3) == True
assert is_odd(7) == True
assert is_even(5) == False

# Recursive sum
def rsum(lst):
    if len(lst) == 0:
        return 0
    return lst[0] + rsum(lst[1:])

assert rsum([1, 2, 3, 4, 5]) == 15
assert rsum([]) == 0

# Recursive flatten
def flatten(lst):
    result = []
    for item in lst:
        if isinstance(item, list):
            result = result + flatten(item)
        else:
            result.append(item)
    return result

assert flatten([1, [2, 3], [4, [5, 6]]]) == [1, 2, 3, 4, 5, 6]
