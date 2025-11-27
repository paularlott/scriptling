import re

# Test re.sub with Python-compatible signature: sub(pattern, repl, string)
text = re.sub("[0-9]+", "XXX", "Price: 100")
text == "Price: XXX"
