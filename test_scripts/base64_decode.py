import base64

encoded = "SGVsbG8sIFdvcmxkIQ=="
decoded = base64.b64decode(encoded)
decoded == "Hello, World!"