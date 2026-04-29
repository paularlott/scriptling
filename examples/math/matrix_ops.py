import math

# Linear Algebra Example: Matrix operations
# Demonstrates math.matmul, math.transpose, math.mat_add, math.dot

# Matrix multiplication
A = [[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]]  # 2x3
B = [[7.0, 8.0], [9.0, 10.0], [11.0, 12.0]]  # 3x2

C = math.matmul(A, B)  # 2x2
print(f"A (2x3) @ B (3x2) = {C}")
# Expected: [[58, 64], [139, 154]]

# Transpose
T = math.transpose(A)  # 3x2
print(f"\nTranspose of A (2x3) -> {len(T)}x{len(T[0])}")
for row in T:
    print(f"  {row}")

# Verify: A @ B should equal transpose(B^T @ A^T)
T2 = math.transpose(math.matmul(math.transpose(B), math.transpose(A)))
print(f"\nVerification (transpose(B^T @ A^T)): {T2}")
assert C[0][0] == T2[0][0]

# Element-wise addition
D = [[10.0, 20.0], [30.0, 40.0]]
E = [[1.0, 2.0], [3.0, 4.0]]
F = math.mat_add(D, E)
print(f"\nD + E = {F}")

# Dot product and norms
v = [3.0, 4.0]
norm = math.sqrt(math.dot(v, v))
print(f"\nNorm of {v} = {norm}")  # 5.0

# Outer product via matmul
x = [1.0, 2.0, 3.0]
y = [4.0, 5.0]
outer = math.matmul(math.transpose([x]), [y])
print(f"\nOuter product: {outer}")
