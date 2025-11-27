import re

parts = re.split("[,;]", "one,two;three")
len(parts) == 3