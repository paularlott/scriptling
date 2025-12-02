# Example: HTTP/Requests library

import requests

print("HTTP Library")

# GET request (increase timeout to avoid flaky failures in CI/network)
response = requests.get("http://127.0.0.1:9000/get", {"timeout": 10})
print(f"GET status: {response.status_code}")
print(f"Content: {response.text}")
print("\n")

# POST request
import json
body = json.dumps({"test": "data"})
response = requests.post("http://127.0.0.1:9000/post", body, {"timeout": 10})
print(f"POST status: {response.status_code}")

# Test response attributes
print(f"Response has text: {len(response.text) > 0}")
print(f"Response has headers: {len(response.headers) > 0}")

