# Example: Requests methods (json, raise_for_status)

import requests

print("Requests Methods")

# Test response.json() method
response = requests.get("https://httpbin.org/json", {"timeout": 10})
# Retry once if we didn't get JSON (sometimes httpbin returns HTML under rate limits)
if response.status_code != 200:
    import time
    time.sleep(1)
    response = requests.get("https://httpbin.org/json", {"timeout": 10})

if response.status_code != 200:
    # Print diagnostic but continue the test suite; report body for debugging
    print(f"warning: expected 200 from /json, got {response.status_code}")
    print(f"body starts: {response.text[:200]}")
else:
    data = response.json()
    print(f"response.json() works: {len(data) > 0}")

# Test raise_for_status() with success
try:
    response = requests.get("https://httpbin.org/status/200", {"timeout": 10})
    response.raise_for_status()
    print("raise_for_status() passed for 200")
except Exception as e:
    print(f"Unexpected error: {e}")

# Test raise_for_status() with 4xx error
try:
    response = requests.get("https://httpbin.org/status/404", {"timeout": 10})
    response.raise_for_status()
    print("Should not reach here")
except Exception as e:
    print(f"raise_for_status() caught 404: {e}")

# Test raise_for_status() with 5xx error
try:
    response = requests.get("https://httpbin.org/status/500", {"timeout": 10})
    response.raise_for_status()
    print("Should not reach here")
except Exception as e:
    print(f"raise_for_status() caught 500: {e}")

# Test exception handling with requests.HTTPError
try:
    response = requests.get("https://httpbin.org/status/403", {"timeout": 10})
    response.raise_for_status()
except requests.HTTPError as e:
    print(f"Caught HTTPError: {e}")

