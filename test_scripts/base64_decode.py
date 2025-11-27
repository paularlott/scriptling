import base64

encoded = "SGVsbG8sIFdvcmxkIQ=="
decoded = base64.decode(encoded)
decoded == "Hello, World!"