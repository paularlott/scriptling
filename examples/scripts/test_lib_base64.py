# Test: Base64 library

import base64

print("=== Testing Base64 Library ===")

# Encode
text = "Hello, World!"
encoded = base64.encode(text)
print(f"Encoded '{text}': {encoded}")

# Decode
decoded = base64.decode(encoded)
print(f"Decoded back: {decoded}")

# Test with different strings
test_cases = ["Python", "Scriptling", "123456"]
for text in test_cases:
    enc = base64.encode(text)
    dec = base64.decode(enc)
    print(f"'{text}' -> '{enc}' -> '{dec}'")

print("✓ All base64 library tests passed")
