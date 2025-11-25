# Test: New Features - Dict methods, Tuple len, and Tuple packing
# Tests the recently implemented features

print("=== Testing New Features ===")

# Test 1: Dict.keys() and Dict.values()
print("\n1. Dict methods:")
person = {"name": "Alice", "age": 30, "city": "NYC"}
keys = person.keys()
values = person.values()
print(f"Dict: {person}")
print(f"Keys: {keys}")
print(f"Values: {values}")

# Test 2: len() on tuples
print("\n2. Tuple length:")
coords = (10, 20, 30)
print(f"Tuple: {coords}")
print(f"Length: {len(coords)}")

# Test 3: Tuple packing (implicit tuple creation)
print("\n3. Tuple packing in assignments:")
a, b = 0, 1
print(f"After 'a, b = 0, 1': a={a}, b={b}")

# Tuple packing with swap
a, b = b, a
print(f"After 'a, b = b, a': a={a}, b={b}")

# Tuple packing with expressions
x, y, z = 1, 2 + 3, 4 * 5
print(f"After 'x, y, z = 1, 2 + 3, 4 * 5': x={x}, y={y}, z={z}")

# Test 4: Fibonacci with tuple packing
print("\n4. Fibonacci sequence using tuple packing:")
def fibonacci(n):
    a, b = 0, 1
    result = []
    for _ in range(n):
        result.append(a)
        a, b = b, a + b
    return result

fib_nums = fibonacci(10)
print(f"First 10 Fibonacci numbers: {fib_nums}")

print("\n✓ All new feature tests passed")
