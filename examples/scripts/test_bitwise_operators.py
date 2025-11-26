# Test bitwise operations
# This tests all bitwise operators: ~, &, |, ^, <<, >>

# Test bitwise NOT (~)
print("=== Bitwise NOT (~) ===")
a = 5
print("~5 =", ~a)  # -6 in Python (two's complement)
print("~0 =", ~0)  # -1
print("~-1 =", ~-1)  # 0
print("~10 =", ~10)  # -11

# Test bitwise AND (&)
print("\n=== Bitwise AND (&) ===")
print("12 & 10 =", 12 & 10)  # 8 (1100 & 1010 = 1000)
print("5 & 3 =", 5 & 3)  # 1 (101 & 011 = 001)
print("15 & 7 =", 15 & 7)  # 7 (1111 & 0111 = 0111)
print("0 & 5 =", 0 & 5)  # 0

# Test bitwise OR (|)
print("\n=== Bitwise OR (|) ===")
print("12 | 10 =", 12 | 10)  # 14 (1100 | 1010 = 1110)
print("5 | 3 =", 5 | 3)  # 7 (101 | 011 = 111)
print("8 | 4 =", 8 | 4)  # 12 (1000 | 0100 = 1100)
print("0 | 5 =", 0 | 5)  # 5

# Test bitwise XOR (^)
print("\n=== Bitwise XOR (^) ===")
print("12 ^ 10 =", 12 ^ 10)  # 6 (1100 ^ 1010 = 0110)
print("5 ^ 3 =", 5 ^ 3)  # 6 (101 ^ 011 = 110)
print("15 ^ 15 =", 15 ^ 15)  # 0 (same values = 0)
print("0 ^ 5 =", 0 ^ 5)  # 5

# Test left shift (<<)
print("\n=== Left Shift (<<) ===")
print("1 << 3 =", 1 << 3)  # 8 (multiply by 2^3)
print("5 << 2 =", 5 << 2)  # 20 (multiply by 2^2)
print("7 << 1 =", 7 << 1)  # 14 (multiply by 2)
print("10 << 0 =", 10 << 0)  # 10 (no shift)

# Test right shift (>>)
print("\n=== Right Shift (>>) ===")
print("8 >> 3 =", 8 >> 3)  # 1 (divide by 2^3)
print("20 >> 2 =", 20 >> 2)  # 5 (divide by 2^2)
print("14 >> 1 =", 14 >> 1)  # 7 (divide by 2)
print("10 >> 0 =", 10 >> 0)  # 10 (no shift)
print("7 >> 1 =", 7 >> 1)  # 3 (integer division)

# Test augmented assignment operators
print("\n=== Augmented Assignment ===")
x = 12
x &= 10
print("x = 12; x &= 10; x =", x)  # 8

x = 12
x |= 10
print("x = 12; x |= 10; x =", x)  # 14

x = 12
x ^= 10
print("x = 12; x ^= 10; x =", x)  # 6

x = 5
x <<= 2
print("x = 5; x <<= 2; x =", x)  # 20

x = 20
x >>= 2
print("x = 20; x >>= 2; x =", x)  # 5

# Test operator precedence
print("\n=== Operator Precedence ===")
print("5 | 3 & 6 =", 5 | 3 & 6)  # 7 (& has higher precedence than |)
print("5 ^ 3 | 2 =", 5 ^ 3 | 2)  # 6 (| has lower precedence than ^)
print("8 >> 1 + 1 =", 8 >> 1 + 1)  # 2 (+ has higher precedence than >>)
print("2 + 3 << 1 =", 2 + 3 << 1)  # 10 (+ has higher precedence than <<)

# Test combined operations
print("\n=== Combined Operations ===")
mask = 15  # 0b1111
value = 170  # 0b10101010
print("value & mask =", value & mask)  # 10 (0b1010)
print("value | mask =", value | mask)  # 175 (0b10101111)
print("value ^ mask =", value ^ mask)  # 165 (0b10100101)

# Test with negative numbers (Python-compatible)
print("\n=== Negative Numbers ===")
print("-5 & 3 =", -5 & 3)  # 3
print("-5 | 3 =", -5 | 3)  # -5
print("-5 ^ 3 =", -5 ^ 3)  # -8

# Test chaining
print("\n=== Chained Operations ===")
a = 255
b = 15
c = 3
result = a & b | c
print("255 & 15 | 3 =", result)  # 15

# Test with variables
print("\n=== Variable Operations ===")
x = 42
y = 24
print("x & y =", x & y)  # 8
print("x | y =", x | y)  # 58
print("x ^ y =", x ^ y)  # 50
print("~x =", ~x)  # -43

print("\n=== All tests completed ===")
