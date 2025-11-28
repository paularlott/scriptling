import urllib.parse

# Test quote
quoted = urllib.parse.quote("hello world")

# Test quote_plus
quoted_plus = urllib.parse.quote_plus("hello world")

# Test basic functionality
quoted == "hello%20world" and quoted_plus == "hello+world"
