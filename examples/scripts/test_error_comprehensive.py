# Test: Comprehensive error handling

print("=== Testing Comprehensive Error Handling ===")

# Test 1: Exception with as clause
try:
    value = 10 / 0
except Exception as e:
    print(f"Test 1 - Caught exception with 'as': {e}")

# Test 2: Multiple try blocks
try:
    print("Test 2 - First try block")
except Exception as e:
    print(f"Unexpected error: {e}")

try:
    print("Test 2 - Second try block")
except Exception as e:
    print(f"Unexpected error: {e}")

# Test 3: Function with error handling
def divide(a, b):
    try:
        return a / b
    except Exception as e:
        print(f"Test 3 - Division error: {e}")
        return 0

result = divide(10, 2)
print(f"Test 3 - divide(10, 2) = {result}")

result = divide(10, 0)
print(f"Test 3 - divide(10, 0) = {result}")

# Test 4: Finally always executes
counter = 0
try:
    counter = 1
    print(f"Test 4 - Try block, counter = {counter}")
finally:
    counter = 2
    print(f"Test 4 - Finally block, counter = {counter}")

print(f"Test 4 - After block, counter = {counter}")

print("✓ All comprehensive error handling tests passed")
