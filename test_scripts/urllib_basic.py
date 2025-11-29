import urllib.parse

# Test quote
quoted = urllib.parse.quote("hello world")
assert quoted == "hello%20world"

# Test quote_plus
quoted_plus = urllib.parse.quote_plus("hello world")
assert quoted_plus == "hello+world"

# Test unquote
encoded = "hello%20world"
decoded = urllib.parse.unquote(encoded)
assert decoded == "hello world"

True