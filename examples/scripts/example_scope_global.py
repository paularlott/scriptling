# Example: Global scope

print("Global Scope")

counter = 0

def increment():
    global counter
    counter = counter + 1

def get_counter():
    global counter
    return counter

print(f"Initial counter: {counter}")
increment()
print(f"After increment(): {get_counter()}")
increment()
increment()
print(f"After 2 more increments: {get_counter()}")

