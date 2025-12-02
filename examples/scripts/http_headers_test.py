import requests
import json

# Test HTTP with headers
print("Testing HTTP with headers...")

# GET with headers
options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123", "User-Agent": "Scriptling/1.0", "Accept": "application/json"}
}
response = requests.get("http://127.0.0.1:9000/headers", options)
if response.status_code == 200:
    data = json.loads(response.body)
    print("Request headers were sent successfully")
else:
    print("GET request failed:", response.status_code)

# POST with headers and custom content type
post_options = {
    "timeout": 10,
    "headers": {"Authorization": "Bearer token123", "Content-Type": "application/json"}
}

payload = {"name": "test", "value": 42}
body = json.dumps(payload)

response = requests.post("http://127.0.0.1:9000/post", body, post_options)
if response.status_code == 200:
    print("POST with headers successful")
else:
    print("POST request failed:", response.status_code)