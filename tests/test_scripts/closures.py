# Counter factory
def make_counter(start=0):
    count = start
    def inc():
        nonlocal count
        count += 1
        return count
    def dec():
        nonlocal count
        count -= 1
        return count
    def reset():
        nonlocal count
        count = start
    return inc, dec, reset

inc, dec, reset = make_counter(10)
assert inc() == 11
assert inc() == 12
assert dec() == 11
reset()
assert inc() == 11

# Independent closures don't share state
inc1, _, _ = make_counter(0)
inc2, _, _ = make_counter(100)
assert inc1() == 1
assert inc2() == 101
assert inc1() == 2
assert inc2() == 102

# Closure captures variable by reference, not value
def make_adder(n):
    def add(x):
        return x + n
    return add

add5 = make_adder(5)
add10 = make_adder(10)
assert add5(3) == 8
assert add10(3) == 13

# Memoisation via closure
def memoize(fn):
    cache = {}
    def wrapper(n):
        if n not in cache:
            cache[n] = fn(n)
        return cache[n]
    return wrapper

call_count = 0

def expensive(n):
    global call_count
    call_count += 1
    return n * n

fast = memoize(expensive)
assert fast(4) == 16
assert fast(4) == 16  # cached
assert fast(5) == 25
assert call_count == 2  # only called twice, not three times

# Closure in a loop — each closure captures its own n via make_adder
adders = [make_adder(i) for i in range(5)]
assert adders[0](10) == 10
assert adders[3](10) == 13
assert adders[4](10) == 14

# Decorator as closure
def repeat(times):
    def decorator(fn):
        def wrapper(*args):
            result = None
            for i in range(times):
                result = fn(*args)
            return result
        return wrapper
    return decorator

@repeat(3)
def say(msg):
    return msg

assert say("hi") == "hi"
