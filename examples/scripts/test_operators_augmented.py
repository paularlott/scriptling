# Test: Augmented assignment operators (+=, -=, *=, /=, etc.)

print("=== Testing Augmented Assignment Operators ===")

# +=
x = 10
x += 5
print(f"After x += 5: {x}")

# -=
x -= 3
print(f"After x -= 3: {x}")

# *=
x *= 2
print(f"After x *= 2: {x}")

# /=
x /= 4
print(f"After x /= 4: {x}")

# String concatenation with +=
text = "Hello"
text += " World"
print(f"After text += ' World': {text}")

print("✓ All augmented assignment tests passed")
