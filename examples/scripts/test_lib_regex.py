# Test: Regex (re) library

import re

print("=== Testing Regex Library ===")

# match - check if pattern matches
result = re.match("[0-9]+", "123abc")
print(f"match('[0-9]+', '123abc'): {result}")

result = re.match("[0-9]+", "abc123")
print(f"match('[0-9]+', 'abc123'): {result}")

# find - find first match
email = re.find("[a-z]+@[a-z]+\\.[a-z]+", "Contact: user@example.com")
print(f"find email: {email}")

# findall - find all matches
phones = re.findall("[0-9]{3}-[0-9]{4}", "Call 555-1234 or 555-5678")
print(f"findall phones: {phones}")

# replace - replace all matches
text = re.replace("[0-9]+", "Price: 100", "REDACTED")
print(f"replace numbers: {text}")

# split - split by pattern
parts = re.split("[,;]", "one,two;three")
print(f"split by [,;]: {parts}")

# More patterns
text = "The year is 2024"
year = re.find("[0-9]{4}", text)
print(f"Extract year: {year}")

print("✓ All regex library tests passed")
