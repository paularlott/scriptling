import urllib.parse

# Test quote_plus
text = "hello world"
encoded = urllib.parse.quote_plus(text)
assert encoded == "hello+world"

# Test unquote_plus
decoded = urllib.parse.unquote_plus(encoded)
assert decoded == text

True