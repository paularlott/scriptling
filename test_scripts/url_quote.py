import urllib.parse

text = "hello world"
encoded = urllib.parse.quote(text)
encoded == "hello%20world"