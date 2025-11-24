# Comprehensive scope management test (global and nonlocal)

print("=== Scope Management Test ===")
print("")

# Test 1: Global vs local scope
print("1. Global vs local scope")
value = 100

def test_local():
    value = 200
    print("   Inside function (local):", value)

test_local()
print("   Outside function (global):", value)
print("")

# Test 2: Global keyword
print("2. Global keyword")
counter = 0

def increment_global():
    global counter
    counter = counter + 1

increment_global()
increment_global()
print("   Counter after 2 increments:", counter)
print("")

# Test 3: Nonlocal keyword
print("3. Nonlocal keyword")
def make_counter():
    count = 0
    def increment():
        nonlocal count
        count = count + 1
        return count
    return increment

inc = make_counter()
print("   First call:", inc())
print("   Second call:", inc())
print("   Third call:", inc())
print("")

# Test 4: Global and nonlocal together
print("4. Global and nonlocal together")
global_var = 0

def outer():
    local_var = 10
    def inner():
        global global_var
        nonlocal local_var
        global_var = global_var + 1
        local_var = local_var + 1
    inner()
    print("   Local var after inner():", local_var)

outer()
print("   Global var after outer():", global_var)
print("")

# Test 5: Multiple globals
print("5. Multiple globals")
x = 1
y = 2
z = 3

def modify_all():
    global x, y, z
    x = x * 10
    y = y * 10
    z = z * 10

modify_all()
print("   x =", x, ", y =", y, ", z =", z)
print("")

# Test 6: Closure with nonlocal
print("6. Closure with nonlocal")
def make_adder(n):
    def add(x):
        nonlocal n
        n = n + x
        return n
    return add

adder = make_adder(10)
print("   Add 5:", adder(5))
print("   Add 3:", adder(3))
print("   Add 2:", adder(2))
print("")

# Test 7: Global in nested functions
print("7. Global in nested functions")
result = 0

def level1():
    def level2():
        def level3():
            global result
            result = 999
        level3()
    level2()

level1()
print("   Result:", result)
print("")

# Test 8: Practical example - accumulator
print("8. Practical example - accumulator")
total = 0

def add_to_total(value):
    global total
    total = total + value
    return total

print("   Add 10:", add_to_total(10))
print("   Add 20:", add_to_total(20))
print("   Add 30:", add_to_total(30))
print("   Final total:", total)
print("")

print("=== All Scope Management Tests Complete ===")
