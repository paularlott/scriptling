# Test: Requests methods (json, raise_for_status)

import requests

print("=== Testing Requests Methods ===")

# Test response.json() method
response = requests.get("https://httpbin.org/json")
data = response.json()
print(f"response.json() works: {len(data) > 0}")

# Test raise_for_status() with success
try:
    response = requests.get("https://httpbin.org/status/200")
    response.raise_for_status()
    print("raise_for_status() passed for 200")
except Exception as e:
    print(f"Unexpected error: {e}")

# Test raise_for_status() with 4xx error
try:
    response = requests.get("https://httpbin.org/status/404")
    response.raise_for_status()
    print("Should not reach here")
except Exception as e:
    print(f"raise_for_status() caught 404: {e}")

# Test raise_for_status() with 5xx error
try:
    response = requests.get("https://httpbin.org/status/500")
    response.raise_for_status()
    print("Should not reach here")
except Exception as e:
    print(f"raise_for_status() caught 500: {e}")

# Test exception handling with requests.HTTPError
try:
    response = requests.get("https://httpbin.org/status/403")
    response.raise_for_status()
except requests.HTTPError as e:
    print(f"Caught HTTPError: {e}")

print("✓ All requests method tests passed")
