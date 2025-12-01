#!/usr/bin/env scriptling

# Test script for the ** (power) operator

print("Testing ** power operator...")

# Test 1: Basic integer exponentiation
print("Test 1: Basic integer exponentiation")
print("2**3 =", 2**3, "(expected 8)")
print("3**2 =", 3**2, "(expected 9)")
print("10**0 =", 10**0, "(expected 1)")
print("5**1 =", 5**1, "(expected 5)")

# Test 2: Larger exponents
print("\nTest 2: Larger exponents")
print("2**10 =", 2**10, "(expected 1024)")
print("3**4 =", 3**4, "(expected 81)")
print("10**3 =", 10**3, "(expected 1000)")

# Test 3: Negative exponents (should return float)
print("\nTest 3: Negative exponents")
print("2**-1 =", 2**-1, "(expected 0.5)")
print("10**-2 =", 10**-2, "(expected 0.01)")
print("5**-1 =", 5**-1, "(expected 0.2)")

# Test 4: Float exponentiation
print("\nTest 4: Float exponentiation")
print("2.0**3 =", 2.0**3, "(expected 8.0)")
print("4.0**0.5 =", 4.0**0.5, "(expected 2.0)")
print("9.0**0.5 =", 9.0**0.5, "(expected 3.0)")

# Test 5: Zero base
print("\nTest 5: Zero base")
print("0**5 =", 0**5, "(expected 0)")
print("0**100 =", 0**100, "(expected 0)")

# Test 6: Power in expressions
print("\nTest 6: Power in expressions")
result = (2 + 3)**2
print("(2 + 3)**2 =", result, "(expected 25)")
result = 2**3 + 3**2
print("2**3 + 3**2 =", result, "(expected 17)")

# Test 7: Power with variables
print("\nTest 7: Power with variables")
base = 5
exp = 2
print("5**2 =", base**exp, "(expected 25)")

# Test 8: Operator precedence
print("\nTest 8: Operator precedence")
print("2 * 3**2 =", 2 * 3**2, "(expected 18 - ** has higher precedence)")
print("2 + 3**2 =", 2 + 3**2, "(expected 11 - ** has higher precedence)")
print("(2 + 3)**2 =", (2 + 3)**2, "(expected 25 - parentheses override)")

# Test 9: List comprehension with power
print("\nTest 9: List comprehension with power")
squares = [x**2 for x in [1, 2, 3, 4, 5]]
print("Squares:", squares, "(expected [1, 4, 9, 16, 25])")

# Test 10: Chained power
print("\nTest 10: Chained power")
print("2**3**2 =", 2**3**2, "(expected 512)")

print("\n✓ All ** power operator tests completed!")

