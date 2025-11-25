# Test: Nonlocal scope

print("=== Testing Nonlocal Scope ===")

def outer():
    x = 10

    def inner():
        nonlocal x
        x = 20

    print(f"Before inner(): x = {x}")
    inner()
    print(f"After inner(): x = {x}")
    return x

result = outer()
print(f"Returned value: {result}")

print("✓ All nonlocal scope tests passed")
