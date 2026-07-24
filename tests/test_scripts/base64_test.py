import base64

# Test base64 encoding
text = "Hello, World!"
encoded = base64.b64encode(text)
assert encoded == "SGVsbG8sIFdvcmxkIQ=="

# Test base64 decoding — b64decode now returns a Bytes value
decoded = base64.b64decode(encoded)
assert decoded.decode() == "Hello, World!"
assert decoded == bytes("Hello, World!")
