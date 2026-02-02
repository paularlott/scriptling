import hashlib

# Test hashlib md5
text = "Hello, World!"
hash_md5 = hashlib.md5(text)
assert hash_md5 == "65a8e27d8879283831b664bd8b7f0ad4"

# Test hashlib sha1
hash_sha1 = hashlib.sha1(text)
assert len(hash_sha1) == 40

# Test hashlib sha256
hash_sha256 = hashlib.sha256(text)
assert hash_sha256 == "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"

# Test hashlib consistency
hash1 = hashlib.sha256("test")
hash2 = hashlib.sha256("test")
assert hash1 == hash2