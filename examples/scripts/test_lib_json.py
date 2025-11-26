# Test: JSON library

import json

print("=== Testing JSON Library ===")

# Test loads
json_str = '{"name":"Alice","age":30,"active":true}'
data = json.loads(json_str)
print(f"Parsed: {data}")
print(f"Name: {data['name']}")
print(f"Age: {data['age']}")

# Test dumps
obj = {"status": "success", "count": "42"}
result = json.dumps(obj)
print(f"Stringified: {result}")

# Test with arrays
json_array = '[1,2,3,4,5]'
arr = json.loads(json_array)
print(f"Array: {arr}")

# Test nested
nested_json = '{"user":{"name":"Bob","scores":[10,20,30]}}'
nested = json.loads(nested_json)
print(f"Nested name: {nested['user']['name']}")
print(f"First score: {nested['user']['scores'][0]}")

print("✓ All JSON library tests passed")
