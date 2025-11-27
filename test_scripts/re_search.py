import re

# Test re.search - finds pattern anywhere in string
result = re.search("[0-9]+", "abc123def")
result == "123"
