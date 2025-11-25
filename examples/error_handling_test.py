# Test error handling features

print("=== Error Handling Tests ===")
print("")

# Test 1: Basic try/except
print("1. Basic try/except")
try:
    x = 10 / 0
    print("This should not print")
except:
    print("   Caught division by zero")
print("")

# Test 2: Try/except with successful code
print("2. Try/except with successful code")
try:
    x = 10 / 2
    print("   Result: " + str(x))
except:
    print("   This should not print")
print("")

# Test 3: Try/finally
print("3. Try/finally")
try:
    x = 5 + 5
    print("   Calculation: " + str(x))
finally:
    print("   Cleanup executed")
print("")

# Test 4: Try/except/finally
print("4. Try/except/finally")
try:
    result = 10 / 0
except:
    print("   Error caught")
    result = 0
finally:
    print("   Finally block executed")
print("   Result: " + str(result))
print("")

# Test 5: Raise statement
print("5. Raise statement")
def check_positive(n):
    if n < 0:
        raise "Value must be positive"
    return n * 2

try:
    result = check_positive(-5)
except:
    print("   Caught raised error")
print("")

# Test 6: Nested try/except
print("6. Nested try/except")
try:
    try:
        x = 10 / 0
    except:
        print("   Inner exception caught")
        raise "Re-raising error"
except:
    print("   Outer exception caught")
print("")

print("=== All Error Handling Tests Complete ===")
