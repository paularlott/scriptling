# Basic nonlocal
def outer():
    x = 10
    def inner():
        nonlocal x
        x = 20
    inner()
    return x

assert outer() == 20

# Nonlocal with augmented assignment
def make_counter():
    count = 0
    def inc():
        nonlocal count
        count += 1
        return count
    return inc

c = make_counter()
assert c() == 1
assert c() == 2
assert c() == 3

# Two independent counters don't share state
c2 = make_counter()
assert c2() == 1
assert c() == 4  # c continues from where it left off

# Deep nonlocal chain (3 levels)
def level_a():
    x = 1
    def level_b():
        nonlocal x
        x = 2
        def level_c():
            nonlocal x
            x = 3
        level_c()
    level_b()
    return x

assert level_a() == 3

# Nonlocal read without write
def outer2():
    msg = "hello"
    def inner2():
        return msg  # read-only capture
    return inner2()

assert outer2() == "hello"

# Closure over loop variable via nonlocal
def make_adder(n):
    def add(x):
        return x + n
    return add

add5 = make_adder(5)
add10 = make_adder(10)
assert add5(3) == 8
assert add10(3) == 13
assert add5(add10(1)) == 16
