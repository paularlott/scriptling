import base64

text = "Hello, World!"
encoded = base64.b64encode(text)
encoded == "SGVsbG8sIFdvcmxkIQ=="