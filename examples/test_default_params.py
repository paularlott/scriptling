# Default Parameter Values Test

# Function with default parameter
def greet(name, greeting="Hello"):
    return greeting + " " + name

# Test with both parameters
result1 = greet("Alice", "Hi")
print("With both params:", result1)

# Test with default parameter
result2 = greet("Bob")
print("With default:", result2)

# Function with multiple defaults
def create_user(name, age=25, active=True):
    return {"name": name, "age": age, "active": active}

# Test various combinations
user1 = create_user("Alice")
print("User1:", user1)

user2 = create_user("Bob", 30)
print("User2:", user2)

user3 = create_user("Charlie", 35, False)
print("User3:", user3)

# Function with mixed parameters
def calculate(x, y, operation="add"):
    if operation == "add":
        return x + y
    elif operation == "multiply":
        return x * y
    else:
        return 0

print("Add default:", calculate(5, 3))
print("Multiply:", calculate(5, 3, "multiply"))

print("Default parameters test completed")