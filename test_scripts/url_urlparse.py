import url

url_str = "https://example.com/path?query=value"
parsed = url.urlparse(url_str)
parsed["scheme"] == "https" and parsed["host"] == "example.com" and parsed["path"] == "/path" and parsed["query"] == "query=value"