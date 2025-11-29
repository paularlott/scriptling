import urllib.parse

encoded = "hello%20world"
decoded = urllib.parse.unquote(encoded)
assert decoded == "hello world"

True