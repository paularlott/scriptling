# Test base64 library

import base64

print("=== Base64 Library Test ===")
print("")

# Test 1: Basic encoding
print("1. Basic encoding")
text = "hello"
encoded = base64.encode(text)
print("   Original:", text)
print("   Encoded:", encoded)
print("")

# Test 2: Basic decoding
print("2. Basic decoding")
decoded = base64.decode(encoded)
print("   Decoded:", decoded)
print("   Match:", decoded == text)
print("")

# Test 3: Encode/decode longer text
print("3. Encode/decode longer text")
message = "The quick brown fox jumps over the lazy dog"
enc = base64.encode(message)
dec = base64.decode(enc)
print("   Original:", message)
print("   Roundtrip match:", dec == message)
print("")

# Test 4: Encode/decode with special characters
print("4. Special characters")
special = "Hello, World! 123 @#$%"
enc_special = base64.encode(special)
dec_special = base64.decode(enc_special)
print("   Original:", special)
print("   Encoded:", enc_special)
print("   Decoded:", dec_special)
print("   Match:", dec_special == special)
print("")

# Test 5: Empty string
print("5. Empty string")
empty = ""
enc_empty = base64.encode(empty)
dec_empty = base64.decode(enc_empty)
print("   Encoded empty:", enc_empty)
print("   Decoded empty:", dec_empty)
print("   Match:", dec_empty == empty)
print("")

# Test 6: Numbers as strings
print("6. Numbers as strings")
numbers = "1234567890"
enc_num = base64.encode(numbers)
dec_num = base64.decode(enc_num)
print("   Original:", numbers)
print("   Roundtrip match:", dec_num == numbers)
print("")

# Test 7: Multiple encode/decode
print("7. Multiple encode/decode")
data = "test data"
enc1 = base64.encode(data)
enc2 = base64.encode(enc1)
dec1 = base64.decode(enc2)
dec2 = base64.decode(dec1)
print("   Original:", data)
print("   After double encode/decode:", dec2)
print("   Match:", dec2 == data)
print("")

print("=== All Base64 Tests Complete ===")
