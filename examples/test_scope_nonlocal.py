# Test nonlocal keyword

print("=== Nonlocal Keyword Test ===")
print("")

# Test 1: Basic nonlocal variable
print("1. Basic nonlocal variable")
def outer1():
    x = 10
    def inner():
        nonlocal x
        x = x + 5
    print("   Before inner(): x =", x)
    inner()
    print("   After inner(): x =", x)

outer1()
print("")

# Test 2: Multiple nonlocal variables
print("2. Multiple nonlocal variables")
def outer2():
    a = 1
    b = 2
    def inner():
        nonlocal a, b
        a = a * 10
        b = b * 10
    print("   Before: a =", a, ", b =", b)
    inner()
    print("   After: a =", a, ", b =", b)

outer2()
print("")

# Test 3: Nonlocal with return
print("3. Nonlocal with return")
def outer3():
    count = 0
    def increment():
        nonlocal count
        count = count + 1
        return count
    result1 = increment()
    result2 = increment()
    print("   First call:", result1)
    print("   Second call:", result2)
    print("   Final count:", count)

outer3()
print("")

# Test 4: Nested nonlocal
print("4. Nested nonlocal")
def outer4():
    x = 100
    def middle():
        def inner():
            nonlocal x
            x = x + 50
        inner()
    print("   Before middle(): x =", x)
    middle()
    print("   After middle(): x =", x)

outer4()
print("")

# Test 5: Nonlocal in loop
print("5. Nonlocal in loop")
def outer5():
    total = 0
    def add(n):
        nonlocal total
        total = total + n
    
    for i in range(1, 6):
        add(i)
    
    print("   Final total:", total)

outer5()
print("")

print("=== All Nonlocal Tests Complete ===")
