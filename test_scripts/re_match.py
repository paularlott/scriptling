import re

result = re.match("[0-9]+", "123abc")
result.group(0) == "123"