import datetime

# Current capabilities
print("=== Current datetime library capabilities ===")

# Basic formatting
print("1. Current time:", datetime.now())
print("2. UTC time:", datetime.utcnow()) 
print("3. Today:", datetime.today())
print("4. Custom format:", datetime.now("%A, %B %d, %Y at %I:%M %p"))

# Parsing and formatting
ts = datetime.strptime("2024-01-15 10:30:45", "%Y-%m-%d %H:%M:%S")
print("5. Parsed timestamp:", ts)
print("6. Formatted back:", datetime.strftime("%Y-%m-%d %H:%M:%S", ts))
print("7. From timestamp:", datetime.fromtimestamp(ts))

print("\n=== What might be useful to add ===")
print("8. ISO format would be nice")
print("9. Date-only operations") 
print("10. Time-only operations")
print("11. Basic date arithmetic")
print("12. More format shortcuts")
