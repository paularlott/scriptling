# Test: Combined global and nonlocal scope

print("=== Testing Combined Scope ===")

global_var = 100

def outer():
    outer_var = 50

    def inner():
        global global_var
        nonlocal outer_var

        global_var = 200
        outer_var = 75

        print(f"In inner: global_var = {global_var}, outer_var = {outer_var}")

    print(f"Before inner: global_var = {global_var}, outer_var = {outer_var}")
    inner()
    print(f"After inner: global_var = {global_var}, outer_var = {outer_var}")

outer()
print(f"After outer: global_var = {global_var}")

print("✓ All combined scope tests passed")
