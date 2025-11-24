# Test all new advanced features together
import json

# List comprehension with method calls
numbers = [1, 2, 3, 4, 5]
doubled_strings = [str(x * 2) for x in numbers if x > 2]
print("Doubled strings:", doubled_strings)

# Method chaining
text = "hello world"
result = text.upper().replace("WORLD", "SCRIPTLING")
print("Chained methods:", result)

# Library method calls
data = json.parse('{"items": [1, 2, 3]}')
processed = [x * x for x in data["items"]]
print("Processed data:", processed)

# Complex example
words = ["hello", "world", "scriptling"]
capitalized = [word.upper() for word in words if len(word) > 4]
print("Capitalized long words:", capitalized)

print("All advanced features working!")