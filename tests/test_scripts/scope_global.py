counter = 0

def increment():
    global counter
    counter = counter + 1

def get_counter():
    global counter
    return counter

increment()
increment()
assert get_counter() == 2