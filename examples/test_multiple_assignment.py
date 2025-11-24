# Test multiple assignment (tuple unpacking)
# Priority 4 feature

print("=== Multiple Assignment Test ===")
print("")

# Test 1: Basic two-variable assignment
print("1. Basic two-variable assignment")
a, b = [1, 2]
print("   a, b = [1, 2]")
print("   a =", a)
print("   b =", b)
print("")

# Test 2: Three variables
print("2. Three-variable assignment")
x, y, z = [10, 20, 30]
print("   x, y, z = [10, 20, 30]")
print("   x =", x, ", y =", y, ", z =", z)
print("")

# Test 3: Mixed types
print("3. Mixed types")
name, age, active = ["Alice", 30, True]
print("   name, age, active = ['Alice', 30, True]")
print("   name =", name)
print("   age =", age)
print("   active =", active)
print("")

# Test 4: Swap variables
print("4. Swap variables")
p = 100
q = 200
print("   Before: p =", p, ", q =", q)
p, q = [q, p]
print("   After swap: p =", p, ", q =", q)
print("")

# Test 5: From function return
print("5. From function return")
def get_coordinates():
    return [50, 75]

px, py = get_coordinates()
print("   Coordinates from function:", px, py)
print("")

# Test 6: From expression
print("6. From expression")
first, second = [1 + 1, 2 * 2]
print("   first, second = [1+1, 2*2]")
print("   first =", first, ", second =", second)
print("")

# Test 7: From list slice
print("7. From list slice")
numbers = [10, 20, 30, 40, 50]
slice_result = numbers[1:3]
n1, n2 = slice_result
print("   From numbers[1:3]:", n1, n2)
print("")

# Test 8: Nested unpacking with dict
print("8. From dictionary values")
config = {"width": "800", "height": "600"}
w, h = [config["width"], config["height"]]
print("   width =", w, ", height =", h)
print("")

# Test 9: Multiple swaps
print("9. Multiple swaps")
r, s, t = [1, 2, 3]
print("   Before: r =", r, ", s =", s, ", t =", t)
r, s = [s, r]
s, t = [t, s]
print("   After swaps: r =", r, ", s =", s, ", t =", t)
print("")

# Test 10: Practical use case
print("10. Practical use case - parsing")
def parse_response():
    status = 200
    body = "Success"
    return [status, body]

status_code, message = parse_response()
print("   Status:", status_code)
print("   Message:", message)
print("")

# Test 11: With range
print("11. With range")
range_list = range(2, 4)
start, end = range_list
print("   From range(2, 4):", start, end)
print("")

# Test 12: Chain assignments
print("12. Chain assignments")
data = [100, 200]
val1, val2 = data
sum_result = val1 + val2
diff_result = val2 - val1
sum_val, diff_val = [sum_result, diff_result]
print("   Sum:", sum_val, ", Diff:", diff_val)
print("")

print("=== All Multiple Assignment Tests Complete ===")
