failures = 0

# Bitwise NOT
if ~5 != -6:
    failures += 1
if ~0 != -1:
    failures += 1

# Bitwise AND
if (12 & 10) != 8:
    failures += 1
if (5 & 3) != 1:
    failures += 1

# Bitwise OR
if (12 | 10) != 14:
    failures += 1
if (5 | 3) != 7:
    failures += 1

# Bitwise XOR
if (12 ^ 10) != 6:
    failures += 1
if (5 ^ 3) != 6:
    failures += 1

# Left shift
if (1 << 3) != 8:
    failures += 1
if (5 << 2) != 20:
    failures += 1

# Right shift
if (8 >> 3) != 1:
    failures += 1
if (20 >> 2) != 5:
    failures += 1

# Augmented
x = 12
x &= 10
if x != 8:
    failures += 1

x = 12
x |= 10
if x != 14:
    failures += 1

x = 12
x ^= 10
if x != 6:
    failures += 1

x = 5
x <<= 2
if x != 20:
    failures += 1

x = 20
x >>= 2
if x != 5:
    failures += 1

failures == 0