# Test: Error handling (try/except/finally/raise)

print("=== Testing Error Handling ===")

# Basic try/except
try:
    x = 10 / 0
except Exception as e:
    print(f"Caught division by zero: {e}")

# Try/finally
try:
    print("In try block")
finally:
    print("In finally block")

# Try/except/finally
try:
    result = 5 + 5
    print(f"Result: {result}")
except Exception as e:
    print(f"Error: {e}")
finally:
    print("Cleanup executed")

# Raise statement
try:
    raise "Custom error message"
except Exception as e:
    print(f"Caught raised error: {e}")

# Nested try/except
try:
    try:
        x = 1 / 0
    except Exception as e:
        print(f"Inner exception: {e}")
        raise "Re-raising error"
except Exception as e:
    print(f"Outer exception: {e}")

print("✓ All error handling tests passed")
