# Function definition and calls
def add(a, b):
    return a + b

def greet(name):
    msg = "Hello, " + name
    print(msg)
    return msg

# Call functions
result = add(5, 3)
print(result)

greet("World")

# Recursive function
def factorial(n):
    if n <= 1:
        return 1
    else:
        return n * factorial(n - 1)

fact5 = factorial(5)
print(fact5)
