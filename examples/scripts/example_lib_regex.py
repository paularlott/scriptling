# Example: Regex (re) library

import re

print("Regex Library")

# match - check if pattern matches at beginning
result = re.match("[0-9]+", "123abc")
print(f"match('[0-9]+', '123abc'): {result}")

result = re.match("[0-9]+", "abc123")
print(f"match('[0-9]+', 'abc123'): {result}")

# search - find first match anywhere in string
email = re.search("\\w+@\\w+\\.\\w+", "Contact: user@example.com")
print(f"search email: {email}")

# findall - find all matches
phones = re.findall("[0-9]{3}-[0-9]{4}", "Call 555-1234 or 555-5678")
print(f"findall phones: {phones}")

# sub - replacement (pattern, repl, string, count=0, flags=0)
text = re.sub("[0-9]+", "REDACTED", "Price: 100")
print(f"sub numbers: {text}")

# sub with count parameter - limit replacements
text2 = re.sub("[0-9]+", "X", "a1b2c3d4", 2)
print(f"sub with count=2: {text2}")

# split - split by pattern
parts = re.split("[,;]", "one,two;three")
print(f"split by [,;]: {parts}")

# split with maxsplit parameter
parts2 = re.split("[,;]", "a,b;c;d", 2)
print(f"split with maxsplit=2: {parts2}")

# compile - validate pattern
pattern = re.compile("[0-9]+")
print(f"compile pattern: {pattern}")

# compile with flags
pattern2 = re.compile("hello", re.I)
print(f"compile with IGNORECASE: {pattern2}")

# escape - escape special characters
escaped = re.escape("a.b+c*d?")
print(f"escape 'a.b+c*d?': {escaped}")

# fullmatch - match entire string
result = re.fullmatch("[0-9]+", "123")
print(f"fullmatch('[0-9]+', '123'): {result}")

result = re.fullmatch("[0-9]+", "123abc")
print(f"fullmatch('[0-9]+', '123abc'): {result}")

# Flags - case-insensitive matching
result = re.match("hello", "HELLO world", re.I)
print(f"match with re.I flag: {result}")

result = re.match("hello", "HELLO world", re.IGNORECASE)
print(f"match with re.IGNORECASE flag: {result}")

# Combined flags
pattern3 = re.compile("hello", re.I | re.M)
print(f"compile with combined flags: {pattern3}")

# Extract year from text
text = "The year is 2024"
year = re.search("[0-9]{4}", text)
print(f"Extract year: {year}")

