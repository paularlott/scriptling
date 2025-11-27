import re

escaped = re.escape("a.b+c*d?")
len(escaped) > 5