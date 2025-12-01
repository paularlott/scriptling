# Test hex literal support
print("Testing hex literals...")

# Test basic hex literals
a = 0xFF
assert a == 255

b = 0x10
assert b == 16

c = 0x0
assert c == 0

d = 0x123ABC
assert d == 1194684

# Test hex in expressions
result = 0x10 + 0x20
assert result == 48

# Test hex in function calls
def test_hex(x):
    return x * 2

val = test_hex(0x5)
assert val == 10

print("All hex literal tests passed!")