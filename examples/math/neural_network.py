import math

# Linear Algebra Example: Simple neural network forward pass
# Demonstrates math.dot, math.matmul, math.transpose, math.softmax, math.tanh
# Also shows FloatArray features: list comprehensions, + concat, .tolist(), .shape()

# Input vector (e.g., 3 features)
x = [0.5, -0.3, 0.8]

# Layer 1 weights (4 neurons, 3 inputs each)
W1 = [
    [0.1, -0.2, 0.3],
    [0.4, 0.5, -0.6],
    [-0.7, 0.8, 0.9],
    [0.2, -0.1, 0.4],
]

# Layer 1 biases
b1 = [0.1, -0.2, 0.3, 0.0]

# Layer 2 weights (2 neurons, 4 inputs each)
W2 = [
    [0.3, -0.1, 0.5, -0.2],
    [-0.4, 0.6, -0.3, 0.1],
]

# Layer 2 biases
b2 = [0.1, -0.1]

# --- Forward pass ---

# Layer 1: z1 = W1 @ x + b1
# Compute W1 @ x using matmul (treating x as a 1x3 matrix)
x_matrix = [x]  # shape (1, 3)
z1 = math.matmul(x_matrix, math.transpose(W1))  # shape (1, 4)

# Add bias and apply tanh activation using list comprehension over FloatArray
h1 = [math.tanh(z1[0][i] + b1[i]) for i in range(len(b1))]
print(f"Hidden layer activations: {[round(v, 4) for v in h1]}")

# Layer 2: z2 = W2 @ h1 + b2
h1_matrix = [h1]  # shape (1, 4)
z2 = math.matmul(h1_matrix, math.transpose(W2))  # shape (1, 2)

# Add bias
logits = [z2[0][i] + b2[i] for i in range(len(b2))]
print(f"Logits: {[round(v, 4) for v in logits]}")

# Apply softmax to get probabilities
probs = math.softmax(logits)
print(f"Probabilities: {[round(v, 4) for v in probs]}")
print(f"Prediction: class {probs.index(max(probs))}")

# --- FloatArray features demo ---

# Convert weights to FloatArray for efficient operations
W1_arr = math.array(W1)
print(f"\nW1 shape: {W1_arr.shape()}")

# FloatArray + concatenation (like KV cache appending)
new_row = math.array([[0.3, -0.1, 0.5]])
appended = W1_arr + new_row
print(f"After concatenation: shape {appended.shape()}")

# Convert FloatArray back to list
plain = appended.tolist()
print(f"As plain list: {type(plain)} with {len(plain)} rows")

# --- Dot product example ---
# Cosine similarity between two vectors
a = [1.0, 2.0, 3.0]
b = [4.0, 5.0, 6.0]
dot = math.dot(a, b)
norm_a = math.sqrt(math.dot(a, a))
norm_b = math.sqrt(math.dot(b, b))
cosine_sim = dot / (norm_a * norm_b)
print(f"\nCosine similarity: {round(cosine_sim, 4)}")

# --- Matrix addition example ---
A = [[1.0, 2.0], [3.0, 4.0]]
B = [[5.0, 6.0], [7.0, 8.0]]
C = math.mat_add(A, B)
print(f"\nA + B = {C}")
