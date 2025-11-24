import json
import http

# Test Python-style booleans
active = True
disabled = False
print("Booleans:", active, disabled)

# Test augmented assignment
counter = 10
counter += 5
counter *= 2
counter -= 3
counter /= 2
print("Augmented assignment result:", counter)

# Test elif
score = 85
if score >= 90:
    grade = "A"
elif score >= 80:
    grade = "B"
elif score >= 70:
    grade = "C"
else:
    grade = "F"
print("Grade:", grade)

# Test HTTP with headers
headers = {"User-Agent": "Scriptling/1.0", "Accept": "application/json"}
response = http.get("https://httpbin.org/headers", headers, 10)

if response["status"] == 200:
    data = json.parse(response["body"])
    print("HTTP with headers successful")
    
    # Test augmented assignment with strings
    message = "Features: "
    message += "booleans, "
    message += "augmented assignment, "
    message += "elif, "
    message += "HTTP headers"
    print(message)
else:
    print("HTTP request failed")

print("All new features working!")