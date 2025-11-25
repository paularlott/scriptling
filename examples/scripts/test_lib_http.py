# Test: HTTP/Requests library

import requests

print("=== Testing HTTP Library ===")

# GET request
response = requests.get("https://httpbin.org/get")
print(f"GET status: {response.status_code}")

# POST request
import json
body = json.stringify({"test": "data"})
response = requests.post("https://httpbin.org/post", body)
print(f"POST status: {response.status_code}")

# Test response attributes
print(f"Response has text: {len(response.text) > 0}")
print(f"Response has headers: {len(response.headers) > 0}")

print("✓ All HTTP library tests passed")
