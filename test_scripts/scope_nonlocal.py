def outer():
    x = 10

    def inner():
        nonlocal x
        x = 20

    inner()
    return x

outer() == 20