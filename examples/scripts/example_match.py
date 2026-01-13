# Match Statement Examples

# Basic value matching
print("=== Basic Value Matching ===")
status = 200
match status:
    case 200:
        print("✓ Success")
    case 404:
        print("✗ Not found")
    case 500:
        print("✗ Server error")
    case _:
        print("? Unknown status")

# Type-based matching
print("\n=== Type-Based Matching ===")
for data in [42, "hello", [1, 2, 3], {"key": "value"}]:
    match data:
        case int():
            print(f"Integer: {data}")
        case str():
            print(f"String: {data}")
        case list():
            print(f"List with {len(data)} items")
        case dict():
            print(f"Dictionary with {len(data)} keys")
        case _:
            print(f"Other type: {type(data)}")

# Guard clauses
print("\n=== Guard Clauses ===")
for value in [150, 75, 25]:
    match value:
        case x if x > 100:
            print(f"{value} is large")
        case x if x > 50:
            print(f"{value} is medium")
        case x:
            print(f"{value} is small")

# Structural matching with dictionaries
print("\n=== Structural Matching ===")
responses = [
    {"status": 200, "data": "Success payload"},
    {"error": "Connection timeout"},
    {"status": 404},
]

for response in responses:
    match response:
        case {"status": 200, "data": payload}:
            print(f"✓ Got data: {payload}")
        case {"error": msg}:
            print(f"✗ Error: {msg}")
        case _:
            print("? Unknown response format")

print("\n=== All examples completed ===")
