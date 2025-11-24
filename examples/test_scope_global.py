# Test global keyword

print("=== Global Keyword Test ===")
print("")

# Test 1: Basic global variable
print("1. Basic global variable")
counter = 0

def increment():
    global counter
    counter = counter + 1

print("   Before:", counter)
increment()
print("   After increment():", counter)
increment()
print("   After 2nd increment():", counter)
print("")

# Test 2: Multiple global variables
print("2. Multiple global variables")
x = 10
y = 20

def modify():
    global x, y
    x = x + 5
    y = y + 10

print("   Before: x =", x, ", y =", y)
modify()
print("   After modify(): x =", x, ", y =", y)
print("")

# Test 3: Global in nested function
print("3. Global in nested function")
total = 0

def outer():
    def inner():
        global total
        total = total + 100
    inner()

print("   Before:", total)
outer()
print("   After outer():", total)
print("")

# Test 4: Global with return value
print("4. Global with return value")
result = 0

def calculate(n):
    global result
    result = n * n
    return result

value = calculate(5)
print("   Returned:", value)
print("   Global result:", result)
print("")

# Test 5: Multiple functions using same global
print("5. Multiple functions using same global")
score = 0

def add_points(points):
    global score
    score = score + points

def reset_score():
    global score
    score = 0

print("   Initial:", score)
add_points(10)
print("   After add_points(10):", score)
add_points(5)
print("   After add_points(5):", score)
reset_score()
print("   After reset_score():", score)
print("")

print("=== All Global Tests Complete ===")
