import urllib.parse

url_str = "https://example.com/path?query=value"
parsed = urllib.parse.urlparse(url_str)
parsed["scheme"] == "https" and parsed["netloc"] == "example.com" and parsed["path"] == "/path" and parsed["query"] == "query=value"