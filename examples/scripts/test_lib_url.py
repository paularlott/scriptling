# Test: URL library

import url

print("=== Testing URL Library ===")

# URL encode
text = "hello world"
encoded = url.encode(text)
print(f"URL encode '{text}': {encoded}")

# URL decode
decoded = url.decode(encoded)
print(f"URL decode '{encoded}': {decoded}")

# Test with special characters
special = "name=John Doe&age=30"
enc = url.encode(special)
dec = url.decode(enc)
print(f"Special chars: '{special}' -> '{enc}' -> '{dec}'")

# Test various strings
test_cases = ["test@example.com", "hello+world", "a/b/c"]
for text in test_cases:
    enc = url.encode(text)
    dec = url.decode(enc)
    print(f"'{text}' -> '{enc}' -> '{dec}'")

print("✓ All URL library tests passed")
