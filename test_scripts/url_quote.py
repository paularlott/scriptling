import url

text = "hello world"
encoded = url.quote(text)
encoded == "hello%20world"