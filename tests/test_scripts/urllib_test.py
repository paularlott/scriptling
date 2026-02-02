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

# Test unquote_plus
decoded = urllib.parse.unquote_plus(quoted_plus)
assert decoded == "hello world"

# Test basic functionality
assert urllib.parse.quote("hello world") == "hello%20world"
assert urllib.parse.quote_plus("hello world") == "hello+world"

# Test urlparse
url_str = "https://example.com/path?query=value"
parsed = urllib.parse.urlparse(url_str)
assert parsed["scheme"] == "https"
assert parsed["netloc"] == "example.com"
assert parsed["path"] == "/path"
assert parsed["query"] == "query=value"

# Test geturl method
reconstructed = parsed.geturl()
assert reconstructed == url_str

# Test more complex URL
complex_url = "https://user:pass@example.com:8080/path/to/resource?query=value&other=123#fragment"
parsed_complex = urllib.parse.urlparse(complex_url)
reconstructed_complex = parsed_complex.geturl()
assert reconstructed_complex == complex_url

# Test urlunparse
components = {"scheme": "https", "netloc": "api.example.com", "path": "/v1/users", "query": "limit=10", "fragment": "section1"}
built_url = urllib.parse.urlunparse(components)
assert built_url == "https://api.example.com/v1/users?limit=10#section1"