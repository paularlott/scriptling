import urllib.parse

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

True