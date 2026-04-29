import math
import random

# Test math.tanh
assert math.tanh(0) == 0.0
result = math.tanh(1.0)
assert result > 0.76 and result < 0.77
result = math.tanh(-1.0)
assert result > -0.77 and result < -0.76
result = math.tanh(100)
assert result > 0.99
result = math.tanh(-100)
assert result < -0.99

# Test math.softmax
probs = math.softmax([2.0, 1.0, 0.1])
assert len(probs) == 3
total = probs[0] + probs[1] + probs[2]
assert total > 0.999 and total < 1.001
assert probs[0] > probs[1] and probs[1] > probs[2]

# Test softmax with equal values
eq = math.softmax([1.0, 1.0, 1.0])
assert eq[0] > 0.32 and eq[0] < 0.34
assert eq[0] == eq[1] and eq[1] == eq[2]

# Test math.dot
result = math.dot([1.0, 2.0, 3.0], [4.0, 5.0, 6.0])
assert result == 32.0
result = math.dot([1, 2, 3], [4, 5, 6])
assert result == 32.0
result = math.dot([], [])
assert result == 0.0

# Test math.matmul
a = [[1.0, 2.0], [3.0, 4.0]]
b = [[5.0, 6.0], [7.0, 8.0]]
c = math.matmul(a, b)
assert c[0][0] == 19.0
assert c[0][1] == 22.0
assert c[1][0] == 43.0
assert c[1][1] == 50.0

# Test non-square matmul
a2 = [[1.0, 2.0, 3.0]]
b2 = [[4.0], [5.0], [6.0]]
c2 = math.matmul(a2, b2)
assert len(c2) == 1 and len(c2[0]) == 1
assert c2[0][0] == 32.0

# Test math.transpose
m = [[1.0, 2.0, 3.0], [4.0, 5.0, 6.0]]
t = math.transpose(m)
assert len(t) == 3
assert len(t[0]) == 2
assert t[0][0] == 1.0
assert t[0][1] == 4.0
assert t[1][0] == 2.0
assert t[2][1] == 6.0

# Transpose back should equal original
t2 = math.transpose(t)
assert len(t2) == 2
assert len(t2[0]) == 3
assert t2[0][0] == 1.0
assert t2[1][2] == 6.0

# Test math.mat_add
a3 = [[1.0, 2.0], [3.0, 4.0]]
b3 = [[5.0, 6.0], [7.0, 8.0]]
c3 = math.mat_add(a3, b3)
assert c3[0][0] == 6.0
assert c3[0][1] == 8.0
assert c3[1][0] == 10.0
assert c3[1][1] == 12.0

# Test with integers in mat_add
a4 = [[1, 2], [3, 4]]
b4 = [[5, 6], [7, 8]]
c4 = math.mat_add(a4, b4)
assert c4[0][0] == 6.0
assert c4[1][1] == 12.0

# Test random.choices with weights
random.seed(42)
colors = ["red", "green", "blue"]
result = random.choices(colors, weights=[5.0, 3.0, 1.0], k=10)
assert len(result) == 10
for c in result:
    assert c in colors

# Test random.choices without weights (uniform)
result = random.choices(colors, k=5)
assert len(result) == 5
for c in result:
    assert c in colors

# Test random.choices k=1 default
result = random.choices(colors, weights=[1.0, 1.0, 1.0])
assert len(result) == 1
assert result[0] in colors

# Test random.choices with integers as weights
result = random.choices([10, 20, 30], weights=[1, 2, 3], k=20)
assert len(result) == 20
for v in result:
    assert v in [10, 20, 30]

True
