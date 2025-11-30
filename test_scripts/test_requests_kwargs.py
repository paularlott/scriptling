# Test: Requests API with Keyword Arguments

import requests

# Test GET with kwargs
response = requests.get(url="https://httpbin.org/get", timeout=10, headers={"X-Test": "True"})
assert response.status_code == 200, f"GET should return 200, got {response.status_code}"
assert response.url == "https://httpbin.org/get", "GET url should match"

# Test POST with kwargs (data as kwarg)
response = requests.post(url="https://httpbin.org/post", data='{"foo": "bar"}', headers={"Content-Type": "application/json"})
assert response.status_code == 200, f"POST should return 200, got {response.status_code}"
json_data = response.json()
assert json_data['json']['foo'] == 'bar', "POST data should be preserved"

# Test POST with mixed args (url positional, data positional, others kwargs)
response = requests.post("https://httpbin.org/post", '{"baz": "qux"}', headers={"Content-Type": "application/json"}, timeout=5)
assert response.status_code == 200, f"Mixed POST should return 200, got {response.status_code}"
json_data = response.json()
assert json_data['json']['baz'] == 'qux', "Mixed POST data should be preserved"

# Test PUT with kwargs
response = requests.put(url="https://httpbin.org/put", data='{"update": "true"}', auth=("user", "pass"))
assert response.status_code == 200, f"PUT should return 200, got {response.status_code}"

# Test DELETE with kwargs
response = requests.delete(url="https://httpbin.org/delete", headers={"X-Delete": "True"})
assert response.status_code == 200, f"DELETE should return 200, got {response.status_code}"

# Test PATCH with kwargs
response = requests.patch(url="https://httpbin.org/patch", data='{"patch": "data"}')
assert response.status_code == 200, f"PATCH should return 200, got {response.status_code}"

# Test legacy options dict still works
options = {"timeout": 10, "headers": {"X-Legacy": "True"}}
response = requests.get("https://httpbin.org/get", options)
assert response.status_code == 200, f"Legacy options should work, got {response.status_code}"

# Test positional args still work
response = requests.get("https://httpbin.org/get")
assert response.status_code == 200, f"Positional GET should work, got {response.status_code}"

response = requests.post("https://httpbin.org/post", '{"test": "data"}')
assert response.status_code == 200, f"Positional POST should work, got {response.status_code}"

print("✓ All requests kwargs tests passed")
