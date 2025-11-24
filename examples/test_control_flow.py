# Control Flow Tests

# Test if/elif/else
x = 15
if x > 20:
    print("Large")
elif x > 10:
    print("Medium")
else:
    print("Small")

# Test nested if statements
y = 5
if y > 0:
    if y < 10:
        print("Single digit positive")
    else:
        print("Multi digit positive")
else:
    print("Not positive")

# Test boolean expressions
a = True
b = False
if a and not b:
    print("Logic test passed")

# Test comparison operators
if 5 == 5 and 3 != 4 and 10 > 5 and 2 < 8 and 5 >= 5 and 3 <= 3:
    print("All comparisons passed")

# Test complex conditions
age = 25
has_license = True
if age >= 18 and has_license:
    print("Can drive")

# Test truthiness
empty_list = []
if not empty_list:
    print("Empty list is falsy")

non_empty_list = [1, 2, 3]
if non_empty_list:
    print("Non-empty list is truthy")

print("Control flow tests completed")