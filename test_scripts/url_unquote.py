import url

encoded = "hello%20world"
decoded = url.unquote(encoded)
decoded == "hello world"