counter = 0

def increment():
    global counter
    counter = counter + 1

def reset():
    global counter
    counter = 0

def get():
    global counter
    return counter

increment()
increment()
increment()
assert get() == 3

reset()
assert get() == 0

# Multiple globals
x = 10
y = 20

def swap():
    global x, y
    x, y = y, x

swap()
assert x == 20
assert y == 10

# Global modified in nested function
total = 0

def add(n):
    global total
    total += n

add(5)
add(3)
add(2)
assert total == 10
