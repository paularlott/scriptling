# Example: Default parameters

print("Default Parameters")

# Function with default parameter
def greet(name, greeting="Hello"):
    return greeting + ", " + name

# Call with both parameters
result = greet("Alice", "Hi")
print(f"greet('Alice', 'Hi'): {result}")

# Call with only required parameter
result = greet("Bob")
print(f"greet('Bob'): {result}")

# Multiple defaults
def create_user(name, age=0, active=True):
    return {"name": name, "age": age, "active": active}

# All defaults
user1 = create_user("Charlie")
print(f"create_user('Charlie'): {user1}")

# Override first default
user2 = create_user("Diana", 25)
print(f"create_user('Diana', 25): {user2}")

# Override both defaults
user3 = create_user("Eve", 30, False)
print(f"create_user('Eve', 30, False): {user3}")

