# Test keyword arguments

def greet(name, greeting="Hello"):
    print(greeting + ", " + name + "!")

print("--- Basic Keyword Arguments ---")
# Positional
greet("World")

# Keyword
greet(name="Scriptling")
greet(greeting="Hi", name="User")

# Mixed
greet("Friend", greeting="Welcome")

print("\n--- Defaults and Keywords ---")
def add(a, b=10, c=20):
    return a + b + c

print("add(5) =", add(5))
print("add(5, c=30) =", add(5, c=30))
print("add(5, b=5) =", add(5, b=5))
print("add(a=1, b=2, c=3) =", add(a=1, b=2, c=3))

print("\n--- Complex Mixed ---")
def complex(x, y=0, z=0):
    return x * 100 + y * 10 + z

print("complex(1, z=5) =", complex(1, z=5)) # 105
print("complex(1, y=2) =", complex(1, y=2)) # 120
print("complex(z=3, x=4) =", complex(z=3, x=4)) # 403

print("\n✓ Keyword arguments tests passed")
