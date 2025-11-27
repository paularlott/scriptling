import re

phones = re.findall("[0-9]{3}-[0-9]{4}", "Call 555-1234 or 555-5678")
len(phones) == 2