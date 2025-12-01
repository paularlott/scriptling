# Example: Using the base64 library in Scriptling
# This demonstrates encoding and decoding data using base64

import base64

print("Base64 Encoding and Decoding Example")

# Encode a string to base64
text = "Hello, World!"
encoded = base64.b64encode(text)
print(f"Encoded '{text}': {encoded}")

# Decode it back
decoded = base64.b64decode(encoded)
print(f"Decoded back: {decoded}")

# Test with different strings
print("\nTesting with various strings:")
test_cases = ["Python", "Scriptling", "123456"]
for text in test_cases:
    enc = base64.b64encode(text)
    dec = base64.b64decode(enc)
    print(f"'{text}' -> '{enc}' -> '{dec}'")
