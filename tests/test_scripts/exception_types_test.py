# Test exception type matching

print("Test 1: Bare except catches all")
try:
    x = 1 / 0
except:
    print("  ✓ Caught with bare except")

print("\nTest 2: except Exception as e catches all")
try:
    y = 1 / 0
except Exception as e:
    print(f"  ✓ Caught with Exception: {e}")

print("\nTest 3: Specific exception type (should not catch)")
caught = False
try:
    try:
        z = 1 / 0
    except ValueError as e:
        caught = True
        print(f"  ✗ Should not catch ValueError")
except:
    print("  ✓ ValueError didn't match, outer except caught it")

print("\nTest 4: Exception variable binding with raise")
try:
    raise Exception("custom message")
except Exception as e:
    print(f"  ✓ Exception variable: {e}")

print("\nAll tests complete!")
