# Test semicolon as statement separator

# Basic semicolon usage
x = 1; y = 2; z = 3
assert x == 1, f"Expected 1, got {x}"
assert y == 2, f"Expected 2, got {y}"
assert z == 3, f"Expected 3, got {z}"

# Semicolons in assignment
a = 10; b = 20
assert a + b == 30

# Multiple statements with semicolons
result = 0; result = result + 1; result = result + 2
assert result == 3, f"Expected 3, got {result}"

# Semicolon after last statement (should be ignored)
c = 100;
assert c == 100

# Mixed newlines and semicolons
d = 1
e = 2; f = 3
g = 4
assert d + e + f + g == 10

print("All semicolon tests passed!")
True
