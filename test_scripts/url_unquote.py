import urllib.parse

encoded = "hello%20world"
decoded = urllib.parse.unquote(encoded)
decoded == "hello world"