# Local scope: variable doesn't leak out
def test_local():
    x = 10
    return x

assert test_local() == 10

# Local shadows global
y = 100
def shadow():
    y = 999
    return y

assert shadow() == 999
assert y == 100  # global unchanged

# Local variables are independent across calls
def make_val(n):
    result = n * 2
    return result

assert make_val(3) == 6
assert make_val(5) == 10

# Nested function has its own local scope
def outer():
    x = 1
    def inner():
        x = 2  # new local, doesn't affect outer's x
        return x
    inner_val = inner()
    return inner_val, x

result = outer()
assert result[0] == 2
assert result[1] == 1
