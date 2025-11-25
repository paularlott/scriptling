import requests
import json

# Test HTTP with headers
print("Testing HTTP with headers...")

# GET with headers
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123", "User-Agent": "Scriptling/1.0", "Accept": "application/json"}
}
response = requests.get("https://httpbin.org/headers", options)
if response["status"] == 200:
    data = json.parse(response["body"])
    print("Request headers were sent successfully")
else:
    print("GET request failed:", response["status"])

# POST with headers and custom content type
post_options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123", "Content-Type": "application/json"}
}

payload = {"name": "test", "value": 42}
body = json.stringify(payload)

response = requests.post("https://httpbin.org/post", body, post_options)
if response["status"] == 200:
    print("POST with headers successful")
else:
    print("POST request failed:", response["status"])