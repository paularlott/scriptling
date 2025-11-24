# JSON Library Tests

import json

# Test JSON parsing
data = json.parse('{"name": "Alice", "age": 30}')
print("Parsed name:", data["name"])
print("Parsed age:", data["age"])

# Test JSON stringify
obj = {"key": "value", "number": 42}
json_str = json.stringify(obj)
print("JSON string:", json_str)

# Test nested objects
nested = {"user": {"name": "Bob", "settings": {"theme": "dark"}}}
nested_str = json.stringify(nested)
parsed_nested = json.parse(nested_str)
print("Nested theme:", parsed_nested["user"]["settings"]["theme"])

# Test arrays
arr = [1, 2, 3, "four"]
arr_str = json.stringify(arr)
parsed_arr = json.parse(arr_str)
print("Array length:", len(parsed_arr))
print("Array item:", parsed_arr[3])

print("JSON library tests completed")