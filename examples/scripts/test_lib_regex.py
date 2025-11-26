# Test: Regex (re) library

import re

print("=== Testing Regex Library ===")

# match - check if pattern matches at beginning
result = re.match("[0-9]+", "123abc")
print(f"match('[0-9]+', '123abc'): {result}")

result = re.match("[0-9]+", "abc123")
print(f"match('[0-9]+', 'abc123'): {result}")

# find - find first match
email = re.find("user@example.com", "Contact: user@example.com")
print(f"find email: {email}")

# search - same as find in Scriptling
email2 = re.search("user@example.com", "Contact: user@example.com")
print(f"search email: {email2}")

# findall - find all matches
phones = re.findall("[0-9]{3}-[0-9]{4}", "Call 555-1234 or 555-5678")
print(f"findall phones: {phones}")

# replace - replace all matches
text = re.replace("[0-9]+", "Price: 100", "REDACTED")
print(f"replace numbers: {text}")

# split - split by pattern
parts = re.split("[,;]", "one,two;three")
print(f"split by [,;]: {parts}")

# compile - validate pattern
pattern = re.compile("[0-9]+")
print(f"compile pattern: {pattern}")

# escape - escape special characters
escaped = re.escape("a.b+c*d?")
print(f"escape 'a.b+c*d?': {escaped}")

# fullmatch - match entire string
result = re.fullmatch("[0-9]+", "123")
print(f"fullmatch('[0-9]+', '123'): {result}")

result = re.fullmatch("[0-9]+", "123abc")
print(f"fullmatch('[0-9]+', '123abc'): {result}")

# More patterns
text = "The year is 2024"
year = re.find("[0-9]{4}", text)
print(f"Extract year: {year}")

print("✓ All regex library tests passed")
