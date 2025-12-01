# Example: Hashlib library

import hashlib

print("Hashlib Library")

# MD5
text = "Hello, World!"
hash_md5 = hashlib.md5(text)
print(f"MD5 of '{text}': {hash_md5}")

# SHA1
hash_sha1 = hashlib.sha1(text)
print(f"SHA1 of '{text}': {hash_sha1}")

# SHA256
hash_sha256 = hashlib.sha256(text)
print(f"SHA256 of '{text}': {hash_sha256}")

# Test consistency
hash1 = hashlib.sha256("test")
hash2 = hashlib.sha256("test")
print(f"Hash consistency: {hash1 == hash2}")

# Test different inputs
texts = ["apple", "banana", "cherry"]
for text in texts:
    h = hashlib.sha256(text)
    print(f"SHA256('{text}'): {h}")

