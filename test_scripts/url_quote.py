import urllib.parse

text = "hello world"
encoded = urllib.parse.quote(text)
assert encoded == "hello%20world"

True