# Test elif functionality
score = 85

if score >= 90:
    print("Grade: A")
elif score >= 80:
    print("Grade: B")
elif score >= 70:
    print("Grade: C")
elif score >= 60:
    print("Grade: D")
else:
    print("Grade: F")

# Test multiple conditions
x = 15

if x < 10:
    print("Small")
elif x < 20:
    print("Medium")
elif x < 30:
    print("Large")
else:
    print("Extra Large")

# Test nested elif
weather = "sunny"
temp = 75

if weather == "rainy":
    print("Stay inside")
elif weather == "sunny":
    if temp > 80:
        print("Perfect beach weather!")
    elif temp > 60:
        print("Nice day for a walk")
    else:
        print("A bit chilly but sunny")
else:
    print("Check the weather")