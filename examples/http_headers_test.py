import("http")
import("json")

# Test HTTP with headers
headers = {"Authorization": "Bearer token123", "User-Agent": "Scriptling/1.0", "Accept": "application/json"}

print("Testing HTTP with headers...")

# GET with headers
response = http.get("https://httpbin.org/headers", headers, 10)
if response["status"] == 200:
    data = json.parse(response["body"])
    print("Request headers were sent successfully")
else:
    print("GET request failed:", response["status"])

# POST with headers and custom content type
post_headers = {"Authorization": "Bearer token123", "Content-Type": "application/json"}

payload = {"name": "test", "value": 42}
body = json.stringify(payload)

response = http.post("https://httpbin.org/post", body, post_headers, 10)
if response["status"] == 200:
    print("POST with headers successful")
else:
    print("POST request failed:", response["status"])