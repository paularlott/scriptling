# String Methods as Methods Test

# Test upper() method
text = "hello world"
upper_text = text.upper()
print("Upper:", upper_text)

# Test lower() method
caps_text = "HELLO WORLD"
lower_text = caps_text.lower()
print("Lower:", lower_text)

# Test split() method
csv_text = "one,two,three"
parts = csv_text.split(",")
print("Split:", parts)

# Test replace() method
original = "hello world"
replaced = original.replace("world", "scriptling")
print("Replace:", replaced)

# Test chaining methods
chained = "Hello World".lower().replace("world", "scriptling")
print("Chained:", chained)

print("String methods test completed")