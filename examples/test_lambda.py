# Lambda Functions Test

# Simple lambda
square = lambda x: x * x
print("Square of 5:", square(5))

# Lambda with multiple parameters
add = lambda a, b: a + b
print("Add 3 + 4:", add(3, 4))

# Lambda with default parameter
greet = lambda name, greeting="Hello": greeting + " " + name
print("Default greeting:", greet("Alice"))
print("Custom greeting:", greet("Bob", "Hi"))

# Lambda in list operations
numbers = [1, 2, 3, 4, 5]
squared = []
for n in numbers:
    append(squared, square(n))
print("Squared numbers:", squared)

# Lambda for filtering (manual)
is_even = lambda x: x % 2 == 0
evens = []
for n in numbers:
    if is_even(n):
        append(evens, n)
print("Even numbers:", evens)

# Lambda with string operations
upper_and_exclaim = lambda text: text.upper() + "!"
print("Excited text:", upper_and_exclaim("hello world"))

# Nested lambda usage
multiply = lambda x: lambda y: x * y
double = multiply(2)
triple = multiply(3)
print("Double 7:", double(7))
print("Triple 7:", triple(7))

print("Lambda functions test completed")