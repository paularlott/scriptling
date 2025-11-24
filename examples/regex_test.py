import re

print("=== Regex Match ===")
if re.match("[0-9]+", "abc123"):
    print("Found digits")

print("\n=== Regex Find ===")
email = "Contact: user@example.com for info"
result = re.find("[a-z]+@[a-z]+\.[a-z]+", email)
print("Email:", result)

print("\n=== Regex Find All ===" )
text = "Call 555-1234 or 555-5678"
phones = re.findall("[0-9]{3}-[0-9]{4}", text)
print("Phones:", phones)

print("\n=== Regex Replace ===")
text = "Price: 100 and 200"
result = re.replace("[0-9]+", text, "REDACTED")
print("Redacted:", result)

print("\n=== Regex Split ===")
text = "one,two;three:four"
parts = re.split("[,;:]", text)
print("Parts:", parts)
for part in parts:
    print("  -", part)
