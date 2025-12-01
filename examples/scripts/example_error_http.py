# Example: Error handling with HTTP requests

import requests

print("Error Handling with HTTP")

# Test successful request
try:
    response = requests.get("https://httpbin.org/status/200")
    print(f"Test 1 - Status: {response.status_code}")
except Exception as e:
    print(f"Test 1 - Error: {e}")

# Test error with raise_for_status
try:
    response = requests.get("https://httpbin.org/status/404")
    response.raise_for_status()
    print("Test 2 - Should not reach here")
except Exception as e:
    print(f"Test 2 - Caught 404 error: {e}")

# Test with custom exception
try:
    response = requests.get("https://httpbin.org/status/500")
    response.raise_for_status()
    print("Test 3 - Should not reach here")
except requests.HTTPError as e:
    print(f"Test 3 - Caught HTTPError: {e}")

