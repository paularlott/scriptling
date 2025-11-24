# Comprehensive Error Handling Examples
# Demonstrates try/except/finally and raise statements

print("=== Comprehensive Error Handling Examples ===")
print("")

# Example 1: Basic try/except
print("1. Basic try/except - Division by zero")
try:
    x = 10 / 0
    print("   This should not print")
except:
    print("   ✓ Caught division by zero error")
print("")

# Example 2: Try/except with successful execution
print("2. Try/except with successful execution")
try:
    x = 10 / 2
    print("   ✓ Result: " + str(x))
except:
    print("   This should not print")
print("")

# Example 3: Try/finally without error
print("3. Try/finally without error")
cleanup_count = 0
try:
    x = 5 + 5
    print("   ✓ Calculation: " + str(x))
finally:
    cleanup_count = 1
    print("   ✓ Finally block executed")
print("   Cleanup count: " + str(cleanup_count))
print("")

# Example 4: Try/finally with error (error propagates)
print("4. Try/finally with error")
cleanup_count2 = 0
try:
    try:
        x = 10 / 0
    finally:
        cleanup_count2 = 1
        print("   ✓ Finally block executed even with error")
except:
    print("   ✓ Error caught in outer try")
print("   Cleanup count: " + str(cleanup_count2))
print("")

# Example 5: Try/except/finally
print("5. Try/except/finally - All three blocks")
result = 0
cleanup = 0
try:
    result = 10 / 0
except:
    print("   ✓ Exception caught")
    result = -1
finally:
    cleanup = 1
    print("   ✓ Finally block executed")
print("   Result: " + str(result) + ", Cleanup: " + str(cleanup))
print("")

# Example 6: Raise statement with string
print("6. Raise statement with string message")
def check_positive(n):
    if n < 0:
        raise "Value must be positive"
    return n * 2

try:
    result = check_positive(-5)
    print("   This should not print")
except:
    print("   ✓ Caught raised error")
print("")

# Example 7: Raise in function, catch in caller
print("7. Raise in function, catch in caller")
def divide(a, b):
    if b == 0:
        raise "Cannot divide by zero"
    return a / b

try:
    result = divide(10, 0)
except:
    print("   ✓ Caught error from function")
    result = 0
print("   Result: " + str(result))
print("")

# Example 8: Nested try/except
print("8. Nested try/except")
outer_caught = 0
inner_caught = 0
try:
    try:
        x = 10 / 0
    except:
        inner_caught = 1
        print("   ✓ Inner exception caught")
except:
    outer_caught = 1
    print("   This should not print")
print("   Inner: " + str(inner_caught) + ", Outer: " + str(outer_caught))
print("")

# Example 9: Multiple operations with error handling
print("9. Multiple operations with error handling")
def safe_divide(a, b):
    try:
        return a / b
    except:
        return 0

results = []
append(results, safe_divide(10, 2))
append(results, safe_divide(10, 0))
append(results, safe_divide(20, 4))
print("   ✓ Results: " + str(results))
print("")

# Example 10: Error handling with lists
print("10. Error handling with list operations")
def get_item(lst, index):
    try:
        return lst[index]
    except:
        return None

my_list = [1, 2, 3]
print("   Item at 1: " + str(get_item(my_list, 1)))
print("   Item at 10: " + str(get_item(my_list, 10)))
print("")

# Example 11: Error handling with dictionaries
print("11. Error handling with dictionary operations")
def get_value(d, key):
    val = d[key]
    if val == None:
        raise "Key not found: " + key
    return val

config = {"host": "localhost", "port": "8080"}
try:
    host = get_value(config, "host")
    print("   ✓ Host: " + host)
    timeout = get_value(config, "timeout")
except:
    print("   ✓ Caught missing key error")
print("")

# Example 12: Validation with raise
print("12. Input validation with raise")
def validate_age(age):
    if age < 0:
        raise "Age cannot be negative"
    if age > 150:
        raise "Age too high"
    return True

ages = [-5, 25, 200]
for age in ages:
    try:
        validate_age(age)
        print("   ✓ Age " + str(age) + " is valid")
    except:
        print("   ✗ Age " + str(age) + " is invalid")
print("")

# Example 13: Cleanup with finally
print("13. Resource cleanup with finally")
def process_data(should_fail):
    resource_opened = 0
    try:
        resource_opened = 1
        print("   Resource opened")
        if should_fail:
            raise "Processing failed"
        print("   Processing succeeded")
        return True
    except:
        print("   Error during processing")
        return False
    finally:
        if resource_opened == 1:
            print("   ✓ Resource closed")

result1 = process_data(False)
print("   Result: " + str(result1))
print("")
result2 = process_data(True)
print("   Result: " + str(result2))
print("")

# Example 14: Error handling in loops
print("14. Error handling in loops")
numbers = [10, 5, 0, 2]
results = []
for n in numbers:
    try:
        result = 100 / n
        append(results, result)
    except:
        print("   ✗ Error dividing by " + str(n))
        append(results, 0)
print("   ✓ Results: " + str(results))
print("")

# Example 15: Recursive function with error handling
print("15. Recursive function with error handling")
def factorial(n):
    if n < 0:
        raise "Factorial not defined for negative numbers"
    if n == 0:
        return 1
    return n * factorial(n - 1)

try:
    print("   5! = " + str(factorial(5)))
    print("   (-3)! = " + str(factorial(-3)))
except:
    print("   ✓ Caught error in factorial")
print("")

print("=== All Comprehensive Error Handling Examples Complete ===")
