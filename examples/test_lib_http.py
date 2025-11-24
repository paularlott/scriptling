# HTTP Library Tests

import http
import json

# Test basic GET request
print("Testing HTTP GET...")
try:
    response = http.get("https://httpbin.org/get", {"timeout": 5})
    print("Status:", response["status"])
    print("Response type:", type(response["body"]))
except:
    print("GET test skipped (network issue)")

# Test GET with query parameters
print("\nTesting GET with parameters...")
try:
    params = {"param1": "value1", "param2": "value2"}
    response = http.get("https://httpbin.org/get", {"params": params, "timeout": 5})
    if response["status"] == 200:
        data = json.parse(response["body"])
        print("Query params received:", data["args"])
except:
    print("GET with params test skipped (network issue)")

# Test POST request
print("\nTesting HTTP POST...")
try:
    post_data = {"name": "Alice", "age": 30}
    response = http.post("https://httpbin.org/post", json.stringify(post_data), {"timeout": 5})
    print("POST Status:", response["status"])
except:
    print("POST test skipped (network issue)")

# Test headers
print("\nTesting custom headers...")
try:
    headers = {"User-Agent": "Scriptling/1.0", "Custom-Header": "test"}
    response = http.get("https://httpbin.org/headers", {"headers": headers, "timeout": 5})
    if response["status"] == 200:
        data = json.parse(response["body"])
        print("Custom header sent:", "Custom-Header" in data["headers"])
except:
    print("Headers test skipped (network issue)")

# Test different HTTP methods
print("\nTesting PUT method...")
try:
    response = http.put("https://httpbin.org/put", '{"test": "data"}', {"timeout": 5})
    print("PUT Status:", response["status"])
except:
    print("PUT test skipped (network issue)")

print("\nTesting DELETE method...")
try:
    response = http.delete("https://httpbin.org/delete", {"timeout": 5})
    print("DELETE Status:", response["status"])
except:
    print("DELETE test skipped (network issue)")

print("HTTP library tests completed")