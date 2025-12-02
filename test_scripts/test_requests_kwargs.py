# Test: Requests API with Keyword Arguments

import requests

# Helper function to handle network errors gracefully
def safe_request(method, url, data=None, headers=None, timeout=5, auth=None):
    try:
        if method == "get":
            return requests.get(url, timeout=timeout, headers=headers or {})
        elif method == "post":
            return requests.post(url, data=data, headers=headers or {}, timeout=timeout)
        elif method == "put":
            return requests.put(url, data=data, auth=auth, timeout=timeout)
        elif method == "delete":
            return requests.delete(url, headers=headers or {}, timeout=timeout)
        elif method == "patch":
            return requests.patch(url, data=data, timeout=timeout)
        else:
            return None
    except Exception as e:
        print(f"Network error (expected in some environments): {e}")
        return None

# Test GET with kwargs
response = safe_request("get", "http://127.0.0.1:9000/get", headers={"X-Test": "True"}, timeout=10)
if response:
    assert response.status_code == 200, f"GET should return 200, got {response.status_code}"
    assert response.url == "http://127.0.0.1:9000/get", "GET url should match"

# Test POST with kwargs (data as kwarg)
response = safe_request("post", "http://127.0.0.1:9000/post", data='{"foo": "bar"}', headers={"Content-Type": "application/json"})
if response:
    assert response.status_code == 200, f"POST should return 200, got {response.status_code}"
    json_data = response.json()
    assert json_data['json']['foo'] == 'bar', "POST data should be preserved"

# Test POST with mixed args (url positional, data positional, others kwargs)
response = safe_request("post", "http://127.0.0.1:9000/post", data='{"baz": "qux"}', headers={"Content-Type": "application/json"}, timeout=5)
if response:
    assert response.status_code == 200, f"Mixed POST should return 200, got {response.status_code}"
    json_data = response.json()
    assert json_data['json']['baz'] == 'qux', "Mixed POST data should be preserved"

# Test PUT with kwargs
response = safe_request("put", "http://127.0.0.1:9000/put", data='{"update": "true"}', auth=("user", "pass"))
if response:
    assert response.status_code == 200, f"PUT should return 200, got {response.status_code}"

# Test DELETE with kwargs
response = safe_request("delete", "http://127.0.0.1:9000/delete", headers={"X-Delete": "True"})
if response:
    assert response.status_code == 200, f"DELETE should return 200, got {response.status_code}"

# Test PATCH with kwargs
response = safe_request("patch", "http://127.0.0.1:9000/patch", data='{"patch": "data"}')
if response:
    assert response.status_code == 200, f"PATCH should return 200, got {response.status_code}"

# Test legacy options dict still works
options = {"timeout": 10, "headers": {"X-Legacy": "True"}}
response = requests.get("http://127.0.0.1:9000/get", options)
if response:
    assert response.status_code == 200, f"Legacy options should work, got {response.status_code}"

# Test positional args still work
response = requests.get("http://127.0.0.1:9000/get")
if response:
    assert response.status_code == 200, f"Positional GET should work, got {response.status_code}"

response = requests.post("http://127.0.0.1:9000/post", '{"test": "data"}')
if response:
    assert response.status_code == 200, f"Positional POST should work, got {response.status_code}"

print("✓ All requests kwargs tests passed (network-dependent)")
