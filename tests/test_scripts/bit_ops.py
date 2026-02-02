# Bitwise NOT
assert ~5 == -6
assert ~0 == -1

# Bitwise AND
assert (12 & 10) == 8
assert (5 & 3) == 1

# Bitwise OR
assert (12 | 10) == 14
assert (5 | 3) == 7

# Bitwise XOR
assert (12 ^ 10) == 6
assert (5 ^ 3) == 6

# Left shift
assert (1 << 3) == 8
assert (5 << 2) == 20

# Right shift
assert (8 >> 3) == 1
assert (20 >> 2) == 5

# Augmented
x = 12
x &= 10
assert x == 8

x = 12
x |= 10
assert x == 14

x = 12
x ^= 10
assert x == 6

x = 5
x <<= 2
assert x == 20

x = 20
x >>= 2
assert x == 5