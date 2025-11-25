# Test: Requests API compatibility

import requests

print("=== Testing Requests API ===")

# Test response.text attribute
response = requests.get("https://httpbin.org/get")
text_len = len(response.text)
print(f"response.text length: {text_len}")

# Test response.status_code attribute
print(f"response.status_code: {response.status_code}")

# Test response["status_code"] dict access
status = response["status_code"]
print(f"response['status_code']: {status}")

# Test response.headers
headers_exist = len(response.headers) > 0
print(f"response.headers exist: {headers_exist}")

print("✓ All requests API tests passed")
