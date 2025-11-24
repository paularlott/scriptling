# Test hashlib library

import hashlib

print("=== Hashlib Library Test ===")
print("")

# Test 1: SHA256
print("1. SHA256 hashing")
text = "hello"
hash_sha256 = hashlib.sha256(text)
print("   Text:", text)
print("   SHA256:", hash_sha256)
print("   Length:", len(hash_sha256))
print("")

# Test 2: SHA1
print("2. SHA1 hashing")
hash_sha1 = hashlib.sha1(text)
print("   Text:", text)
print("   SHA1:", hash_sha1)
print("   Length:", len(hash_sha1))
print("")

# Test 3: MD5
print("3. MD5 hashing")
hash_md5 = hashlib.md5(text)
print("   Text:", text)
print("   MD5:", hash_md5)
print("   Length:", len(hash_md5))
print("")

# Test 4: Same input produces same hash
print("4. Consistency check")
hash1 = hashlib.sha256("test")
hash2 = hashlib.sha256("test")
print("   Hash 1:", hash1)
print("   Hash 2:", hash2)
print("   Match:", hash1 == hash2)
print("")

# Test 5: Different inputs produce different hashes
print("5. Different inputs")
hash_a = hashlib.sha256("a")
hash_b = hashlib.sha256("b")
print("   Hash of 'a':", hash_a)
print("   Hash of 'b':", hash_b)
print("   Different:", hash_a != hash_b)
print("")

# Test 6: Empty string
print("6. Empty string")
hash_empty = hashlib.sha256("")
print("   SHA256 of empty:", hash_empty)
print("")

# Test 7: Long text
print("7. Long text")
long_text = "The quick brown fox jumps over the lazy dog"
hash_long = hashlib.sha256(long_text)
print("   Text:", long_text)
print("   SHA256:", hash_long)
print("")

# Test 8: All three hash functions
print("8. All three hash functions on same input")
data = "password123"
print("   Data:", data)
print("   SHA256:", hashlib.sha256(data))
print("   SHA1:", hashlib.sha1(data))
print("   MD5:", hashlib.md5(data))
print("")

# Test 9: Numbers as strings
print("9. Numbers as strings")
numbers = "1234567890"
print("   Input:", numbers)
print("   SHA256:", hashlib.sha256(numbers))
print("")

print("=== All Hashlib Tests Complete ===")
