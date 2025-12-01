# Example: Import functionality

print("Import")

# Import single library
import json
data = json.loads('{"test":"value"}')
print(f"json import works: {data['test']}")

# Import multiple libraries
import math
import base64

result = math.sqrt(16)
print(f"math.sqrt(16) = {result}")

encoded = base64.b64encode("test")
print(f"base64.b64encode('test') = {encoded}")

# Import in function
def test_import():
    import hashlib
    h = hashlib.md5("test")
    return h

hash_result = test_import()
print(f"Import in function works: {len(hash_result) > 0}")

