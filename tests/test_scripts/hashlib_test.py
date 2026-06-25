import hashlib

text = "Hello, World!"

# Test hashlib md5
assert hashlib.md5(text).hexdigest() == "65a8e27d8879283831b664bd8b7f0ad4"

# Test hashlib sha1
sha1 = hashlib.sha1(text).hexdigest()
assert len(sha1) == 40
assert sha1 == "0a0a9f2a6772942557ab5355d76af442f8f65e01"

# Test hashlib sha256
sha256 = hashlib.sha256(text).hexdigest()
assert sha256 == "dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"

# Test hashlib consistency
hash1 = hashlib.sha256("test").hexdigest()
hash2 = hashlib.sha256("test").hexdigest()
assert hash1 == hash2

# update() accumulates data
h = hashlib.sha256()
h.update("foo")
h.update("bar")
assert h.hexdigest() == hashlib.sha256("foobar").hexdigest()

# hash object attributes
h = hashlib.sha256("x")
assert h.name == "sha256"
assert h.digest_size == 32
assert h.block_size == 64
assert hashlib.md5("x").digest_size == 16
assert hashlib.sha1("x").digest_size == 20

# copy() is independent
h = hashlib.sha256("foo")
c = h.copy()
h.update("baz")
assert h.hexdigest() == hashlib.sha256("foobaz").hexdigest()
assert c.hexdigest() == hashlib.sha256("foo").hexdigest()

# digest() returns the raw bytes as a string (same value, hex-encoded)
assert hashlib.sha256("abc").hexdigest() == "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"

# constructor accepts a list of byte values (as returned by str.encode())
assert hashlib.sha256("abc".encode()).hexdigest() == hashlib.sha256("abc").hexdigest()
