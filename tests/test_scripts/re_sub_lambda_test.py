import re

# Test re.sub with lambda function
text = "hello world"
result = re.sub(r'(\w+)', lambda m: m.group(1).upper(), text)
assert result == "HELLO WORLD", f"Expected 'HELLO WORLD', got '{result}'"

# Test with inline code formatting (the original use case)
content = "test `code` here and `more` code"
backtick = chr(96)
result = re.sub(backtick + r'([^' + backtick + r']+)' + backtick, lambda m: "[" + m.group(1) + "]", content)
assert result == "test [code] here and [more] code", f"Expected 'test [code] here and [more] code', got '{result}'"

# Test with count parameter
text = "a b c d"
result = re.sub(r'\w', lambda m: m.group(0).upper(), text, 2)
assert result == "A B c d", f"Expected 'A B c d', got '{result}'"

# Test with no matches
text = "hello"
result = re.sub(r'\d+', lambda m: "X", text)
assert result == "hello", f"Expected 'hello', got '{result}'"

# Test with groups
text = "John Doe, Jane Smith"
result = re.sub(r'(\w+) (\w+)', lambda m: m.group(2) + " " + m.group(1), text)
assert result == "Doe John, Smith Jane", f"Expected 'Doe John, Smith Jane', got '{result}'"

print("All re.sub lambda tests passed!")
