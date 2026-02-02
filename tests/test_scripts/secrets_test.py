import secrets

# Test token_bytes
token = secrets.token_bytes(16)
assert len(token) == 16

# Test token_hex
hex_token = secrets.token_hex(16)
assert len(hex_token) == 32  # 16 bytes * 2 hex chars

# Test token_urlsafe
url_token = secrets.token_urlsafe(16)
assert len(url_token) >= 21  # base64 encoded

# Test compare_digest
assert secrets.compare_digest("abc", "abc")
assert not secrets.compare_digest("abc", "def")

# Test randbelow
num = secrets.randbelow(10)
assert 0 <= num < 10

# Test randbits
bits = secrets.randbits(8)
assert 0 <= bits < 256

# Test choice
items = [1, 2, 3, 4, 5]
chosen = secrets.choice(items)
assert chosen in items