# Test: Boolean short-circuit assignment and chained comparisons

print("=== Boolean Short-Circuit Assignment ===")

# or returns first truthy value
config = None
default_config = {"host": "localhost", "port": "8080"}
active_config = config or default_config
print("Active config:", active_config)

# and returns first falsy value or last value
user_input = ""
message = user_input and "You entered: " + user_input
print("Message:", message)

user_input = "hello"
message = user_input and "You entered: " + user_input
print("Message:", message)

print()
print("=== Chained Comparisons ===")

# Range validation
age = 25
if 18 <= age <= 65:
    print("Age", age, "is in working range")

# Multiple comparisons
x = 5
y = 10
z = 15
if x < y < z:
    print("Values are in ascending order:", x, y, z)

# Temperature range
temp = 72
if 60 < temp < 80:
    print("Temperature", temp, "is comfortable")

print()
print("✓ All advanced feature tests passed")
