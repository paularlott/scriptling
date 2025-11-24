# Error Handling with HTTP and JSON
# Demonstrates real-world error handling patterns

import json
import http

print("=== Error Handling with HTTP and JSON ===")
print("")

# Example 1: Basic HTTP error handling
print("1. Basic HTTP request with error handling")
def fetch_data(url):
    try:
        options = {"timeout": 5}
        response = http.get(url, options)
        
        if response["status"] != 200:
            raise "HTTP error: " + str(response["status"])
        
        return json.parse(response["body"])
    except:
        print("   ✗ Request failed")
        return None
    finally:
        print("   ✓ Request complete")

# This will fail gracefully
result = fetch_data("https://httpbin.org/status/404")
print("   Result: " + str(result))
print("")

# Example 2: JSON parsing with error handling
print("2. JSON parsing with error handling")
def safe_parse_json(text):
    try:
        return json.parse(text)
    except:
        print("   ✗ Invalid JSON")
        return None

valid_json = '{"name": "Alice", "age": 30}'
invalid_json = '{invalid json}'

data1 = safe_parse_json(valid_json)
print("   Valid JSON parsed: " + str(data1 != None))

data2 = safe_parse_json(invalid_json)
print("   Invalid JSON handled: " + str(data2 == None))
print("")

# Example 3: Retry logic with error handling
print("3. Retry logic with error handling")
def fetch_with_retry(url, max_retries):
    retries = 0
    while retries < max_retries:
        try:
            options = {"timeout": 2}
            response = http.get(url, options)
            
            if response["status"] == 200:
                return json.parse(response["body"])
            
            raise "HTTP " + str(response["status"])
        except:
            retries = retries + 1
            print("   Retry " + str(retries) + "/" + str(max_retries))
    
    return None

# This demonstrates retry logic (will fail but show retries)
result = fetch_with_retry("https://httpbin.org/status/500", 3)
print("   Final result: " + str(result))
print("")

# Example 4: Validate response structure
print("4. Validate response structure")
def get_user_name(user_id):
    try:
        url = "https://jsonplaceholder.typicode.com/users/" + str(user_id)
        options = {"timeout": 5}
        response = http.get(url, options)
        
        if response["status"] != 200:
            raise "HTTP error"
        
        user = json.parse(response["body"])
        name = user["name"]
        
        if name == None:
            raise "Missing name field"
        
        return name
    except:
        return "Unknown"
    finally:
        print("   ✓ Request processed")

name = get_user_name(1)
print("   User name: " + name)
print("")

# Example 5: Multiple API calls with error handling
print("5. Multiple API calls with error handling")
def fetch_multiple_users(user_ids):
    results = []
    errors = 0
    
    for user_id in user_ids:
        try:
            url = "https://jsonplaceholder.typicode.com/users/" + str(user_id)
            options = {"timeout": 5}
            response = http.get(url, options)
            
            if response["status"] == 200:
                user = json.parse(response["body"])
                append(results, user["name"])
            else:
                raise "HTTP error"
        except:
            errors = errors + 1
            append(results, None)
    
    return results

user_ids = [1, 2, 999]
names = fetch_multiple_users(user_ids)
print("   ✓ Fetched " + str(len(names)) + " results")
print("")

# Example 6: POST with error handling
print("6. POST request with error handling")
def create_resource(data):
    try:
        body = json.stringify(data)
        headers = {"Content-Type": "application/json"}
        options = {"timeout": 5, "headers": headers}
        response = http.post("https://jsonplaceholder.typicode.com/posts", body, options)
        
        if response["status"] != 201:
            raise "Create failed: " + str(response["status"])
        
        return json.parse(response["body"])
    except:
        print("   ✗ Failed to create resource")
        return None
    finally:
        print("   ✓ POST request complete")

new_post = {"title": "Test", "body": "Content", "userId": "1"}
result = create_resource(new_post)
print("   Created: " + str(result != None))
print("")

# Example 7: Timeout handling
print("7. Timeout handling")
def fetch_with_timeout(url, timeout):
    try:
        options = {"timeout": timeout}
        response = http.get(url, options)
        return response["status"]
    except:
        print("   ✗ Request timed out or failed")
        return 0

# Very short timeout likely to fail
status = fetch_with_timeout("https://httpbin.org/delay/5", 1)
print("   Status: " + str(status))
print("")

# Example 8: Error handling with cleanup
print("8. Error handling with cleanup")
def process_api_data(url):
    connection_open = False
    try:
        connection_open = True
        print("   Opening connection...")
        
        options = {"timeout": 5}
        response = http.get(url, options)
        
        if response["status"] != 200:
            raise "HTTP error"
        
        data = json.parse(response["body"])
        print("   ✓ Data processed")
        return data
    except:
        print("   ✗ Error processing data")
        return None
    finally:
        if connection_open:
            print("   ✓ Connection closed")

result = process_api_data("https://jsonplaceholder.typicode.com/users/1")
print("")

print("=== All HTTP Error Handling Examples Complete ===")
