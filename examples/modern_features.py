# Modern Scriptling Features Demo

print("=== None Support ===")
result = None
print("result:", result)

if result == None:
    print("result is None")

# None is falsy
if not result:
    print("None is falsy")

print("\n=== True Division ===")
print("5 / 2 =", 5 / 2)
print("10 / 4 =", 10 / 4)
print("5 % 2 =", 5 % 2)

print("\n=== Modern Library API ===")
import json

# JSON with dot notation
data = json.parse('{"name":"Alice","age":30}')
print("Parsed:", data)
print("Name:", data["name"])

json_str = json.stringify({"status": "success", "count": 42})
print("Stringified:", json_str)

print("\n=== HTTP with Options Dictionary ===")
import http

# Simple request (5 second default timeout)
print("Making HTTP request...")
response = http.get("https://httpbin.org/status/200")
print("Status:", response["status"])

# With options
options = {"timeout": 10, "headers": {"User-Agent": "Scriptling/1.0"}}
response = http.get("https://httpbin.org/headers", options)
if response["status"] == 200:
    print("Request with options successful")

print("\n=== All Features Working! ===")
