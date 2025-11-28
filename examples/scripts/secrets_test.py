# Test secrets library (extended)
import secrets

# Test token_hex
token = secrets.token_hex(16)
len(token) == 32

# Test token_urlsafe
token = secrets.token_urlsafe(16)
len(token) > 0

# Test token_bytes
bytes = secrets.token_bytes(8)
len(bytes) == 8

# Test randbelow
num = secrets.randbelow(100)
num >= 0 and num < 100

# Test choice
items = ["apple", "banana", "cherry"]
item = secrets.choice(items)
item in items

# Test compare_digest
secrets.compare_digest("hello", "hello")
not secrets.compare_digest("hello", "world")
