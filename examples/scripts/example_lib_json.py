# Example: Using the JSON library in Scriptling
# Demonstrates parsing JSON strings and converting objects to JSON

import json

print("JSON Parsing and Serialization Example")

# Parse a JSON string into a Python object
json_str = '{"name":"Alice","age":30,"active":true}'
data = json.loads(json_str)
print(f"Parsed JSON: {data}")
print(f"Accessing fields - Name: {data['name']}, Age: {data['age']}")

# Convert a Python object to JSON string
obj = {"status": "success", "count": 42}
result = json.dumps(obj)
print(f"Serialized to JSON: {result}")

# Work with JSON arrays
json_array = '[1,2,3,4,5]'
arr = json.loads(json_array)
print(f"Parsed array: {arr}")

# Handle nested JSON structures
nested_json = '{"user":{"name":"Bob","scores":[10,20,30]}}'
nested = json.loads(nested_json)
print(f"Nested access - Name: {nested['user']['name']}")
print(f"Array in nested: First score = {nested['user']['scores'][0]}")
