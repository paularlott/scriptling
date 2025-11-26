import datetime

# Common use cases covered by current library:
print("✅ Logging timestamps:", datetime.now())
print("✅ API timestamps:", datetime.now("%Y-%m-%dT%H:%M:%SZ"))
print("✅ User display:", datetime.now("%A, %B %d, %Y at %I:%M %p"))
print("✅ File timestamps:", datetime.fromtimestamp(1705314645.0))
print("✅ Parsing user input:", datetime.strptime("2024-01-15", "%Y-%m-%d"))
print("✅ Date-only display:", datetime.today())

print("\n❓ Missing use cases that might be common:")
print("• ISO 8601 format for APIs")
print("• Date-only parsing/formatting") 
print("• Time-only operations")
print("• Adding/subtracting time periods")
print("• Timezone conversions")
print("• Weekday calculations")
