# Nested function definition
def outer(x):
    def inner(y):
        return x + y
    return inner(10)

assert outer(5) == 15

# Returning inner function (closure)
def make_multiplier(n):
    def multiply(x):
        return x * n
    return multiply

triple = make_multiplier(3)
assert triple(4) == 12
assert triple(7) == 21

# Nested functions with multiple levels
def level1(a):
    def level2(b):
        def level3(c):
            return a + b + c
        return level3
    return level2

fn = level1(1)(2)
assert fn(3) == 6

# Inner function modifying outer via nonlocal
def accumulator():
    total = 0
    def add(n):
        nonlocal total
        total += n
        return total
    return add

acc = accumulator()
assert acc(5) == 5
assert acc(3) == 8
assert acc(2) == 10

# Mutual recursion via nested scope
def is_even_odd(n):
    def is_even(x):
        if x == 0:
            return True
        return is_odd(x - 1)
    def is_odd(x):
        if x == 0:
            return False
        return is_even(x - 1)
    return is_even(n), is_odd(n)

result = is_even_odd(4)
assert result[0] == True
assert result[1] == False

result2 = is_even_odd(7)
assert result2[0] == False
assert result2[1] == True
