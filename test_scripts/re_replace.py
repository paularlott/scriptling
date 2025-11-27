import re

text = re.replace("[0-9]+", "Price: 100", "REDACTED")
text == "Price: REDACTED"